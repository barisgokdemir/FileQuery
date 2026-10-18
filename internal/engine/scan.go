package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
)

// ScanStats summarizes a scan.
type ScanStats struct {
	Entries     int64    // files and directories seen
	Dirs        int64    // directories read
	Denied      int64    // directories that could not be read
	DeniedPaths []string // first few unreadable directories
}

const maxDeniedPaths = 20

type dirJob struct {
	path  string
	depth int // depth of the entries inside this directory
}

// workQueue is a LIFO queue of directories shared by a fixed pool of workers.
// Depth-first order keeps memory use low on very wide trees, and a fixed pool
// means the number of goroutines does not grow with the number of directories.
type workQueue struct {
	mu      sync.Mutex
	cond    *sync.Cond
	jobs    []dirJob
	pending int // pushed but not yet finished
	stopped bool
}

func newWorkQueue() *workQueue {
	q := &workQueue{}
	q.cond = sync.NewCond(&q.mu)
	return q
}

func (q *workQueue) push(j dirJob) {
	q.mu.Lock()
	q.jobs = append(q.jobs, j)
	q.pending++
	q.mu.Unlock()
	q.cond.Signal()
}

// pop blocks until a job is available. It returns false when all work is done
// or the scan was stopped.
func (q *workQueue) pop() (dirJob, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	for len(q.jobs) == 0 && q.pending > 0 && !q.stopped {
		q.cond.Wait()
	}
	if q.stopped || len(q.jobs) == 0 {
		return dirJob{}, false
	}
	j := q.jobs[len(q.jobs)-1]
	q.jobs = q.jobs[:len(q.jobs)-1]
	return j, true
}

func (q *workQueue) done() {
	q.mu.Lock()
	q.pending--
	finished := q.pending == 0
	q.mu.Unlock()
	if finished {
		q.cond.Broadcast()
	}
}

func (q *workQueue) stop() {
	q.mu.Lock()
	q.stopped = true
	q.mu.Unlock()
	q.cond.Broadcast()
}

// ResolveRoot expands ~ and %VARIABLES%, makes the path absolute and checks
// that it is a readable directory.
func ResolveRoot(raw string) (string, error) {
	p := expandPath(strings.TrimSpace(raw))
	// "C:" means the current directory on drive C; users mean the drive root.
	if len(p) == 2 && p[1] == ':' {
		p += string(filepath.Separator)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("'%s': %w", raw, err)
	}
	info, err := os.Stat(abs)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "", fmt.Errorf("'%s' bulunamadı", raw)
	case errors.Is(err, fs.ErrPermission):
		return "", fmt.Errorf("'%s' klasörüne erişim izni yok", raw)
	case err != nil:
		return "", fmt.Errorf("'%s': %w", raw, err)
	case !info.IsDir():
		return "", fmt.Errorf("'%s' bir klasör değil", raw)
	}
	return abs, nil
}

func expandPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = home + p[1:]
		}
	}
	// %USERPROFILE%\Desktop style variables, on every platform. $VAR is not
	// expanded because '$' is common in Windows paths (C:\$Recycle.Bin).
	for {
		start := strings.IndexByte(p, '%')
		if start < 0 {
			break
		}
		end := strings.IndexByte(p[start+1:], '%')
		if end < 0 {
			break
		}
		name := p[start+1 : start+1+end]
		val, ok := os.LookupEnv(name)
		if !ok {
			break
		}
		p = p[:start] + val + p[start+2+end:]
	}
	return p
}

// Scan walks the given root directories with a pool of workers and calls
// visit for every entry. visit is called concurrently and must be safe for
// that; returning false stops the scan (used for LIMIT). maxDepth 0 means
// unlimited. If progress is not nil it is incremented for every entry seen,
// so callers can show a live counter.
func Scan(ctx context.Context, roots []string, maxDepth int, progress *atomic.Int64, visit func(*Entry) bool) ScanStats {
	var (
		stats    ScanStats
		deniedMu sync.Mutex
		entries  atomic.Int64
		dirs     atomic.Int64
		denied   atomic.Int64
	)
	if progress == nil {
		progress = new(atomic.Int64)
	}

	q := newWorkQueue()
	for _, r := range roots {
		q.push(dirJob{path: r, depth: 1})
	}

	stopCtx := context.AfterFunc(ctx, q.stop)
	defer stopCtx()

	workers := runtime.NumCPU() * 2
	if workers < 4 {
		workers = 4
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				job, ok := q.pop()
				if !ok {
					return
				}
				dirs.Add(1)
				err := readDir(job.path, func(de fs.DirEntry) bool {
					entries.Add(1)
					progress.Add(1)
					e := newEntry(job.path, de, job.depth)
					if !visit(e) {
						q.stop()
						return false
					}
					if e.IsDir() && (maxDepth == 0 || job.depth < maxDepth) {
						q.push(dirJob{path: e.Path, depth: job.depth + 1})
					}
					return true
				})
				if err != nil {
					denied.Add(1)
					deniedMu.Lock()
					if len(stats.DeniedPaths) < maxDeniedPaths {
						stats.DeniedPaths = append(stats.DeniedPaths, job.path)
					}
					deniedMu.Unlock()
				}
				q.done()
			}
		}()
	}
	wg.Wait()

	stats.Entries = entries.Load()
	stats.Dirs = dirs.Load()
	stats.Denied = denied.Load()
	return stats
}

// readDir lists a directory in batches without sorting, which is noticeably
// faster than os.ReadDir on large directories.
func readDir(path string, fn func(fs.DirEntry) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	for {
		batch, err := f.ReadDir(1024)
		for _, de := range batch {
			if !fn(de) {
				return nil
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

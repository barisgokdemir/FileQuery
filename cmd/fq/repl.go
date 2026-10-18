package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/barisgokdemir/FileQuery/internal/output"
)

const replHelp = `Komutlar:
  .yardim            bu yardım
  .alanlar           kullanılabilecek alanlar
  .bicim tablo|csv|json   çıktı biçimini değiştirir
  .cikis             çıkış (Ctrl+D veya Ctrl+Z de olur)

Sorguyu yazıp Enter'a basın. Birden fazla satıra bölmek için satırı \ ile bitirin.
Uzun süren bir sorguyu Ctrl+C ile durdurabilirsiniz.

Örnek:
  SELECT name, size FROM '~' WHERE ext = 'pdf' ORDER BY size DESC LIMIT 10
`

// repl runs the interactive mode. On Windows the console already provides
// line editing and history (arrow keys, F7), so no extra library is needed.
func repl(stdin io.Reader, stdout, stderr io.Writer, cfg *config) int {
	fmt.Fprintf(stdout, "FileQuery %s, dosya sisteminde SQL ile arama.\n.yardim ile komutları görün, .cikis ile çıkın.\n\n", version)

	sc := bufio.NewScanner(stdin)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)

	// Ctrl+C cancels the running query instead of closing the program.
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)

	var pending strings.Builder
	for {
		if pending.Len() == 0 {
			fmt.Fprint(stdout, "fq> ")
		} else {
			fmt.Fprint(stdout, "..> ")
		}
		if !sc.Scan() {
			fmt.Fprintln(stdout)
			return 0
		}
		line := strings.TrimRight(sc.Text(), " \t\r")

		if strings.HasSuffix(line, `\`) {
			pending.WriteString(strings.TrimSuffix(line, `\`))
			pending.WriteString("\n")
			continue
		}
		pending.WriteString(line)
		src := strings.TrimSpace(pending.String())
		pending.Reset()
		if src == "" {
			continue
		}

		if strings.HasPrefix(src, ".") || isExit(src) {
			if !command(src, stdout, stderr, cfg) {
				return 0
			}
			continue
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			select {
			case <-interrupts:
				cancel()
			case <-done:
			}
		}()
		execute(ctx, src, *cfg, stdout, stderr)
		close(done)
		cancel()
		fmt.Fprintln(stdout)
	}
}

func isExit(s string) bool {
	switch strings.ToLower(strings.TrimSuffix(s, ";")) {
	case "exit", "quit", `\q`:
		return true
	}
	return false
}

// command handles a dot command; it returns false when the REPL should exit.
func command(src string, stdout, stderr io.Writer, cfg *config) bool {
	parts := strings.Fields(src)
	switch strings.ToLower(parts[0]) {
	case ".cikis", ".çıkış", ".exit", ".quit", "exit", "quit", `\q`:
		return false
	case ".yardim", ".yardım", ".help":
		fmt.Fprint(stdout, replHelp)
	case ".alanlar", ".fields":
		printFields(stdout)
	case ".bicim", ".biçim", ".format":
		if len(parts) < 2 {
			fmt.Fprintln(stdout, "şu anki biçim:", cfg.format)
			break
		}
		if _, err := output.New(parts[1], io.Discard); err != nil {
			fmt.Fprintln(stderr, err)
			break
		}
		cfg.format = strings.ToLower(parts[1])
		fmt.Fprintln(stdout, "çıktı biçimi:", cfg.format)
	default:
		fmt.Fprintf(stderr, "bilinmeyen komut %s; .yardim yazın\n", parts[0])
	}
	return true
}

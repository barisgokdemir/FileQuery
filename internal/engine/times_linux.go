//go:build linux

package engine

import (
	"io/fs"
	"time"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

// Linux's stat(2) does not report a creation time; statx(2) does, but is not
// available through the standard library, so created is NULL on Linux.
func createdTime(fs.FileInfo) query.Value { return query.Null }

func accessedTime(info fs.FileInfo) query.Value {
	if st := stat(info); st != nil {
		return query.Time(time.Unix(int64(st.Atim.Sec), int64(st.Atim.Nsec)))
	}
	return query.Null
}

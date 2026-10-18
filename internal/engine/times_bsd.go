//go:build darwin || freebsd || netbsd

package engine

import (
	"io/fs"
	"time"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

func createdTime(info fs.FileInfo) query.Value {
	if st := stat(info); st != nil {
		return query.Time(time.Unix(int64(st.Birthtimespec.Sec), int64(st.Birthtimespec.Nsec)))
	}
	return query.Null
}

func accessedTime(info fs.FileInfo) query.Value {
	if st := stat(info); st != nil {
		return query.Time(time.Unix(int64(st.Atimespec.Sec), int64(st.Atimespec.Nsec)))
	}
	return query.Null
}

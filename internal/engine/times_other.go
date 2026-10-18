//go:build unix && !linux && !darwin && !freebsd && !netbsd

package engine

import (
	"io/fs"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

func createdTime(fs.FileInfo) query.Value  { return query.Null }
func accessedTime(fs.FileInfo) query.Value { return query.Null }

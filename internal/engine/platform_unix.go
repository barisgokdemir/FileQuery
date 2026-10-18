//go:build unix

package engine

import (
	"io/fs"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

type platformInfo struct{}

func stat(info fs.FileInfo) *syscall.Stat_t {
	st, _ := info.Sys().(*syscall.Stat_t)
	return st
}

func isHidden(name string, _ fs.FileInfo) bool { return strings.HasPrefix(name, ".") }
func isReadOnly(info fs.FileInfo) bool         { return info.Mode().Perm()&0o222 == 0 }
func isSystem(fs.FileInfo) bool                { return false }

var (
	userCache  sync.Map // uid -> name
	groupCache sync.Map // gid -> name
)

func ownerOf(_ string, info fs.FileInfo, _ *platformInfo) query.Value {
	st := stat(info)
	if st == nil {
		return query.Null
	}
	id := strconv.FormatUint(uint64(st.Uid), 10)
	if name, ok := userCache.Load(id); ok {
		return query.String(name.(string))
	}
	name := id
	if u, err := user.LookupId(id); err == nil {
		name = u.Username
	}
	userCache.Store(id, name)
	return query.String(name)
}

func groupOf(info fs.FileInfo) query.Value {
	st := stat(info)
	if st == nil {
		return query.Null
	}
	id := strconv.FormatUint(uint64(st.Gid), 10)
	if name, ok := groupCache.Load(id); ok {
		return query.String(name.(string))
	}
	name := id
	if g, err := user.LookupGroupId(id); err == nil {
		name = g.Name
	}
	groupCache.Store(id, name)
	return query.String(name)
}

func linkCount(_ string, info fs.FileInfo) query.Value {
	st := stat(info)
	if st == nil {
		return query.Null
	}
	return query.Int(int64(st.Nlink))
}

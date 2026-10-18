// Package engine scans the file system and evaluates FileQuery queries.
package engine

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

// Entry is one file or directory found during a scan. Expensive attributes
// (file info, owner, link count) are only fetched when a query asks for them.
type Entry struct {
	Path  string
	Name  string
	Dir   string
	Depth int

	dirEntry fs.DirEntry
	info     fs.FileInfo
	infoErr  error
	infoDone bool
	plat     *platformInfo
}

func newEntry(dir string, de fs.DirEntry, depth int) *Entry {
	return &Entry{
		Path:     filepath.Join(dir, de.Name()),
		Name:     de.Name(),
		Dir:      dir,
		Depth:    depth,
		dirEntry: de,
	}
}

// IsDir reports whether the entry is a directory that can be descended into.
// Symbolic links, junctions and other reparse points are never followed, so
// scans cannot loop.
func (e *Entry) IsDir() bool {
	t := e.dirEntry.Type()
	return t.IsDir() && t&(fs.ModeSymlink|fs.ModeIrregular) == 0
}

func (e *Entry) fileInfo() fs.FileInfo {
	if !e.infoDone {
		e.info, e.infoErr = e.dirEntry.Info()
		e.infoDone = true
	}
	if e.infoErr != nil {
		return nil
	}
	return e.info
}

func (e *Entry) platform() *platformInfo {
	if e.plat == nil {
		e.plat = &platformInfo{}
	}
	return e.plat
}

// Get returns the value of a field for this entry.
func (e *Entry) Get(field string) query.Value {
	switch field {
	case "name":
		return query.String(e.Name)
	case "ext":
		if e.dirEntry.IsDir() {
			return query.String("")
		}
		return query.String(strings.ToLower(strings.TrimPrefix(filepath.Ext(e.Name), ".")))
	case "path":
		return query.String(e.Path)
	case "dir":
		return query.String(e.Dir)
	case "depth":
		return query.Int(int64(e.Depth))
	case "type":
		t := e.dirEntry.Type()
		switch {
		case t&(fs.ModeSymlink|fs.ModeIrregular) != 0:
			return query.String("link")
		case t.IsDir():
			return query.String("dir")
		}
		return query.String("file")
	}

	info := e.fileInfo()
	if info == nil {
		return query.Null // the file disappeared or cannot be read
	}

	switch field {
	case "size":
		if info.IsDir() {
			return query.Null
		}
		return query.Int(info.Size())
	case "modified":
		return query.Time(info.ModTime())
	case "mode":
		return query.String(info.Mode().Perm().String()[1:])
	case "created":
		return createdTime(info)
	case "accessed":
		return accessedTime(info)
	case "hidden":
		return query.Bool(isHidden(e.Name, info))
	case "readonly":
		return query.Bool(isReadOnly(info))
	case "system":
		return query.Bool(isSystem(info))
	case "owner":
		return ownerOf(e.Path, info, e.platform())
	case "group":
		return groupOf(info)
	case "links":
		return linkCount(e.Path, info)
	}
	return query.Null
}

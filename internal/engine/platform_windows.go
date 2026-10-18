//go:build windows

package engine

import (
	"io/fs"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

type platformInfo struct{}

func attrData(info fs.FileInfo) *syscall.Win32FileAttributeData {
	d, _ := info.Sys().(*syscall.Win32FileAttributeData)
	return d
}

func filetime(ft syscall.Filetime) query.Value {
	if ft.HighDateTime == 0 && ft.LowDateTime == 0 {
		return query.Null
	}
	return query.Time(time.Unix(0, ft.Nanoseconds()))
}

func createdTime(info fs.FileInfo) query.Value {
	if d := attrData(info); d != nil {
		return filetime(d.CreationTime)
	}
	return query.Null
}

func accessedTime(info fs.FileInfo) query.Value {
	if d := attrData(info); d != nil {
		return filetime(d.LastAccessTime)
	}
	return query.Null
}

func hasAttr(info fs.FileInfo, attr uint32) bool {
	d := attrData(info)
	return d != nil && d.FileAttributes&attr != 0
}

func isHidden(_ string, info fs.FileInfo) bool { return hasAttr(info, syscall.FILE_ATTRIBUTE_HIDDEN) }
func isReadOnly(info fs.FileInfo) bool         { return hasAttr(info, syscall.FILE_ATTRIBUTE_READONLY) }
func isSystem(info fs.FileInfo) bool           { return hasAttr(info, syscall.FILE_ATTRIBUTE_SYSTEM) }

var (
	advapi32                 = syscall.NewLazyDLL("advapi32.dll")
	procGetNamedSecurityInfo = advapi32.NewProc("GetNamedSecurityInfoW")
	ownerCache               sync.Map // SID string -> "DOMAIN\user"
)

const (
	seFileObject             = 1
	ownerSecurityInformation = 0x00000001
)

// ownerOf reads the owner SID from the file's security descriptor and turns it
// into an account name. Names are cached per SID because lookups are slow.
func ownerOf(path string, _ fs.FileInfo, _ *platformInfo) query.Value {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return query.Null
	}
	var owner *syscall.SID
	var sd uintptr
	r, _, _ := procGetNamedSecurityInfo.Call(
		uintptr(unsafe.Pointer(p)),
		seFileObject,
		ownerSecurityInformation,
		uintptr(unsafe.Pointer(&owner)),
		0, 0, 0,
		uintptr(unsafe.Pointer(&sd)),
	)
	if r != 0 || owner == nil {
		return query.Null
	}
	defer syscall.LocalFree(syscall.Handle(sd))

	sidStr, err := owner.String()
	if err != nil {
		return query.Null
	}
	if name, ok := ownerCache.Load(sidStr); ok {
		return query.String(name.(string))
	}
	account, domain, _, err := owner.LookupAccount("")
	name := sidStr
	if err == nil {
		name = account
		if domain != "" {
			name = domain + `\` + account
		}
	}
	ownerCache.Store(sidStr, name)
	return query.String(name)
}

func groupOf(fs.FileInfo) query.Value { return query.Null }

// linkCount opens the file to read its hard link count.
func linkCount(path string, _ fs.FileInfo) query.Value {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return query.Null
	}
	h, err := syscall.CreateFile(p, 0,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil, syscall.OPEN_EXISTING, syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return query.Null
	}
	defer syscall.CloseHandle(h)
	var fi syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(h, &fi); err != nil {
		return query.Null
	}
	return query.Int(int64(fi.NumberOfLinks))
}

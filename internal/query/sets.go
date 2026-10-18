package query

import (
	"sort"
	"strings"
)

// ExtSet is a named list of file extensions usable in queries as @name:
//
//	WHERE ext IN @resim AND ext NOT IN ('ico', 'svg')
//	WHERE ext IN (@resim, @video, 'pdf')
type ExtSet struct {
	Name    string   // canonical (Turkish) name, without @
	Aliases []string // English names
	Desc    string
	Exts    []string // lower case, without dots
}

// ExtSets lists the built-in extension sets in display order.
var ExtSets = []ExtSet{
	{
		Name: "resim", Aliases: []string{"image", "images"},
		Desc: "fotoğraf, grafik, simge, RAW ve tasarım dosyaları",
		Exts: []string{"jpg", "jpeg", "jpe", "jfif", "png", "gif", "bmp", "dib", "tif", "tiff", "webp", "heic", "heif",
			"avif", "jxl", "ico", "cur", "svg", "svgz", "psd", "xcf", "tga", "exr", "hdr", "emf", "wmf", "pcx",
			"raw", "dng", "cr2", "cr3", "nef", "arw", "orf", "rw2", "raf"},
	},
	{
		Name: "video", Aliases: []string{"videos"},
		Desc: "video dosyaları",
		Exts: []string{"mp4", "m4v", "mkv", "mov", "avi", "wmv", "webm", "flv", "f4v", "mpg", "mpeg", "mpe", "m2v",
			"3gp", "3g2", "ts", "mts", "m2ts", "vob", "ogv", "rm", "rmvb", "asf", "divx", "mxf", "dv", "qt"},
	},
	{
		Name: "ses", Aliases: []string{"audio", "muzik", "müzik"},
		Desc: "müzik ve ses kayıtları",
		Exts: []string{"mp3", "wav", "flac", "m4a", "aac", "ogg", "oga", "opus", "wma", "aiff", "aif", "alac", "ape",
			"mid", "midi", "amr", "mka", "wv", "ac3", "dts"},
	},
	{
		Name: "belge", Aliases: []string{"document", "documents"},
		Desc: "Office, PDF, metin ve e-kitap dosyaları",
		Exts: []string{"pdf", "doc", "docx", "docm", "dot", "dotx", "odt", "rtf", "txt", "md", "xls", "xlsx", "xlsm",
			"xlsb", "ods", "csv", "tsv", "ppt", "pptx", "pptm", "pps", "ppsx", "odp", "pages", "numbers", "key",
			"epub", "xps", "djvu"},
	},
	{
		Name: "arsiv", Aliases: []string{"arşiv", "archive", "archives"},
		Desc: "sıkıştırılmış arşivler ve disk kalıpları",
		Exts: []string{"zip", "rar", "7z", "tar", "gz", "tgz", "bz2", "tbz2", "xz", "txz", "zst", "lz", "lzma", "z",
			"cab", "iso", "img", "dmg", "arj", "lzh"},
	},
}

var extSetIndex = func() map[string]*ExtSet {
	m := map[string]*ExtSet{}
	for i := range ExtSets {
		s := &ExtSets[i]
		m[Fold(s.Name)] = s
		for _, a := range s.Aliases {
			m[Fold(a)] = s
		}
	}
	return m
}()

// LookupExtSet finds a set by name or alias, case-insensitively.
func LookupExtSet(name string) (*ExtSet, bool) {
	s, ok := extSetIndex[Fold(strings.TrimPrefix(name, "@"))]
	return s, ok
}

func extSetNames() []string {
	var out []string
	for k := range extSetIndex {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

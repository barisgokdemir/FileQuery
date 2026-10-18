package query

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Error is a query error that knows where in the source it happened.
type Error struct {
	Pos int // byte offset
	Msg string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d. karakter: %s", e.Pos+1, e.Msg)
}

// Format renders the error with the offending query line and a caret under
// the position, the way compilers do:
//
//	SELECT name FROM 'C:\' WHERE size > > 1
//	                                   ^
//	36. karakter: beklenmeyen ">"
func (e *Error) Format(src string) string {
	lineStart := strings.LastIndex(src[:min(e.Pos, len(src))], "\n") + 1
	lineEnd := strings.IndexByte(src[lineStart:], '\n')
	if lineEnd < 0 {
		lineEnd = len(src)
	} else {
		lineEnd += lineStart
	}
	line := src[lineStart:lineEnd]
	col := utf8.RuneCountInString(src[lineStart:min(e.Pos, len(src))])
	return fmt.Sprintf("%s\n%s^\n%s", line, strings.Repeat(" ", col), e.Error())
}

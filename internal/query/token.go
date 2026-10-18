// Package query implements the FileQuery language: a small SQL dialect for
// searching the file system.
package query

import "fmt"

// TokenKind identifies the lexical class of a token.
type TokenKind int

const (
	TokEOF TokenKind = iota
	TokIdent
	TokNumber
	TokString
	TokComma
	TokLParen
	TokRParen
	TokStar
	TokPlus
	TokMinus
	TokEq
	TokNeq
	TokLt
	TokLte
	TokGt
	TokGte
	TokSemicolon
	TokKeyword
	TokIllegal // a character the language does not use; reported by the parser with context
)

// Token is a single lexical unit with its byte offset in the source.
type Token struct {
	Kind TokenKind
	Text string // raw text; for keywords the upper-cased keyword
	Pos  int

	// For numbers: an optional unit suffix written directly after the digits
	// ("2MB", "7d"). Empty when there is none.
	Suffix string
}

func (t Token) String() string {
	switch t.Kind {
	case TokEOF:
		return "sorgu sonu"
	case TokString:
		return fmt.Sprintf("'%s'", t.Text)
	case TokNumber:
		return t.Text + t.Suffix
	default:
		return fmt.Sprintf("%q", t.Text)
	}
}

var keywords = map[string]bool{
	"SELECT": true, "FROM": true, "WHERE": true, "AND": true, "OR": true, "NOT": true,
	"LIKE": true, "MATCHES": true, "IN": true, "BETWEEN": true, "ORDER": true, "BY": true,
	"ASC": true, "DESC": true, "LIMIT": true, "GROUP": true, "AS": true, "DEPTH": true,
	"TRUE": true, "FALSE": true, "NULL": true, "IS": true,
}

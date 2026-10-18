package query

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Lex splits a query into tokens. String literals use single quotes; a quote
// inside a string is written twice ('O”Brien'). Backslashes have no special
// meaning, so Windows paths can be written as-is: 'C:\Users\baris'.
func Lex(src string) ([]Token, error) {
	var toks []Token
	i := 0
	for i < len(src) {
		r, w := utf8.DecodeRuneInString(src[i:])
		switch {
		case unicode.IsSpace(r):
			i += w

		case r == '-' && strings.HasPrefix(src[i:], "--"):
			// Line comment
			for i < len(src) && src[i] != '\n' {
				i++
			}

		case r == '\'' || r == '"':
			start := i
			quote := byte(r)
			i++
			var sb strings.Builder
			closed := false
			for i < len(src) {
				if src[i] == quote {
					if i+1 < len(src) && src[i+1] == quote {
						sb.WriteByte(quote)
						i += 2
						continue
					}
					i++
					closed = true
					break
				}
				sb.WriteByte(src[i])
				i++
			}
			if !closed {
				return nil, &Error{Pos: start, Msg: "kapanmamış metin: tırnak işareti eksik"}
			}
			toks = append(toks, Token{Kind: TokString, Text: sb.String(), Pos: start})

		case unicode.IsDigit(r) || (r == '.' && i+1 < len(src) && isDigit(src[i+1])):
			start := i
			for i < len(src) && (isDigit(src[i]) || src[i] == '.') {
				i++
			}
			num := src[start:i]
			if strings.Count(num, ".") > 1 {
				return nil, &Error{Pos: start, Msg: "geçersiz sayı: " + num}
			}
			sufStart := i
			for i < len(src) {
				r2, w2 := utf8.DecodeRuneInString(src[i:])
				if !unicode.IsLetter(r2) {
					break
				}
				i += w2
			}
			toks = append(toks, Token{Kind: TokNumber, Text: num, Suffix: src[sufStart:i], Pos: start})

		case unicode.IsLetter(r) || r == '_':
			start := i
			for i < len(src) {
				r2, w2 := utf8.DecodeRuneInString(src[i:])
				if !unicode.IsLetter(r2) && !unicode.IsDigit(r2) && r2 != '_' {
					break
				}
				i += w2
			}
			word := src[start:i]
			up := strings.ToUpper(word)
			if keywords[up] {
				toks = append(toks, Token{Kind: TokKeyword, Text: up, Pos: start})
			} else {
				toks = append(toks, Token{Kind: TokIdent, Text: word, Pos: start})
			}

		default:
			start := i
			two := ""
			if i+1 < len(src) {
				two = src[i : i+2]
			}
			switch {
			case two == "<=":
				toks = append(toks, Token{Kind: TokLte, Text: two, Pos: start})
				i += 2
			case two == ">=":
				toks = append(toks, Token{Kind: TokGte, Text: two, Pos: start})
				i += 2
			case two == "!=" || two == "<>":
				toks = append(toks, Token{Kind: TokNeq, Text: two, Pos: start})
				i += 2
			case two == "==":
				toks = append(toks, Token{Kind: TokEq, Text: two, Pos: start})
				i += 2
			default:
				kind, ok := map[rune]TokenKind{
					',': TokComma, '(': TokLParen, ')': TokRParen, '*': TokStar,
					'+': TokPlus, '-': TokMinus, '=': TokEq, '<': TokLt, '>': TokGt, ';': TokSemicolon,
				}[r]
				if !ok {
					kind = TokIllegal
				}
				toks = append(toks, Token{Kind: kind, Text: string(r), Pos: start})
				i += w
			}
		}
	}
	toks = append(toks, Token{Kind: TokEOF, Pos: len(src)})
	return toks, nil
}

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

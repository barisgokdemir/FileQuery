package query

import (
	"regexp"
	"strconv"
	"strings"
)

// Expr is any expression node.
type Expr interface {
	Pos() int
	String() string
}

type (
	Literal struct {
		Value  Value
		pos    int
		source string // original text, e.g. "2 MB", used by String()
	}

	Field struct {
		Name  string // lower-case canonical name
		Alias bool   // refers to a SELECT alias (only allowed in ORDER BY)
		pos   int
	}

	Unary struct {
		Op  string // "NOT" or "-"
		X   Expr
		pos int
	}

	Binary struct {
		Op   string // AND OR = != < <= > >= + -
		L, R Expr
		pos  int
	}

	Like struct {
		X       Expr
		Pattern Expr
		Not     bool
		pos     int
	}

	Matches struct {
		X       Expr
		Pattern Expr
		Not     bool
		pos     int
		Regex   *regexp.Regexp // compiled at parse time when the pattern is a literal
	}

	In struct {
		X    Expr
		List []Expr
		Not  bool
		pos  int
	}

	Between struct {
		X, Lo, Hi Expr
		Not       bool
		pos       int
	}

	IsNull struct {
		X   Expr
		Not bool
		pos int
	}

	Call struct {
		Name string // upper-case
		Args []Expr
		Star bool // COUNT(*)
		pos  int
	}
)

func (e *Literal) Pos() int { return e.pos }
func (e *Field) Pos() int   { return e.pos }
func (e *Unary) Pos() int   { return e.pos }
func (e *Binary) Pos() int  { return e.pos }
func (e *Like) Pos() int    { return e.pos }
func (e *Matches) Pos() int { return e.pos }
func (e *In) Pos() int      { return e.pos }
func (e *Between) Pos() int { return e.pos }
func (e *IsNull) Pos() int  { return e.pos }
func (e *Call) Pos() int    { return e.pos }

func (e *Literal) String() string {
	if e.source != "" {
		return e.source
	}
	return e.Value.String()
}
func (e *Field) String() string { return e.Name }
func (e *Unary) String() string {
	if e.Op == "NOT" {
		return "NOT " + e.X.String()
	}
	return e.Op + e.X.String()
}
func (e *Binary) String() string {
	return "(" + e.L.String() + " " + e.Op + " " + e.R.String() + ")"
}
func (e *Like) String() string {
	return e.X.String() + not(e.Not) + " LIKE " + e.Pattern.String()
}
func (e *Matches) String() string {
	return e.X.String() + not(e.Not) + " MATCHES " + e.Pattern.String()
}
func (e *In) String() string {
	parts := make([]string, len(e.List))
	for i, x := range e.List {
		parts[i] = x.String()
	}
	return e.X.String() + not(e.Not) + " IN (" + strings.Join(parts, ", ") + ")"
}
func (e *Between) String() string {
	return e.X.String() + not(e.Not) + " BETWEEN " + e.Lo.String() + " AND " + e.Hi.String()
}
func (e *IsNull) String() string {
	if e.Not {
		return e.X.String() + " IS NOT NULL"
	}
	return e.X.String() + " IS NULL"
}
func (e *Call) String() string {
	if e.Star {
		return e.Name + "(*)"
	}
	parts := make([]string, len(e.Args))
	for i, x := range e.Args {
		parts[i] = x.String()
	}
	return e.Name + "(" + strings.Join(parts, ", ") + ")"
}

func not(b bool) string {
	if b {
		return " NOT"
	}
	return ""
}

// SelectItem is one output column.
type SelectItem struct {
	Expr  Expr
	Alias string // empty when not given
}

// Name is the column heading.
func (s SelectItem) Name() string {
	if s.Alias != "" {
		return s.Alias
	}
	if f, ok := s.Expr.(*Field); ok {
		return f.Name
	}
	return strings.ToLower(s.Expr.String())
}

// OrderItem is one ORDER BY key.
type OrderItem struct {
	Expr Expr
	Desc bool
}

// Query is a parsed SELECT statement.
type Query struct {
	Star    bool
	Items   []SelectItem
	Sources []string
	Depth   int // maximum depth below a source; 0 means unlimited
	Where   Expr
	GroupBy []Expr
	OrderBy []OrderItem
	Limit   int // 0 means no limit
}

// IsAggregate reports whether an expression contains an aggregate call.
func IsAggregate(e Expr) bool { return firstAggregate(e) != nil }

// firstAggregate returns the first aggregate call inside e, or nil.
func firstAggregate(e Expr) *Call {
	var found *Call
	Walk(e, func(x Expr) {
		if c, ok := x.(*Call); ok && aggregates[c.Name] && found == nil {
			found = c
		}
	})
	return found
}

// Walk visits e and all of its sub-expressions.
func Walk(e Expr, fn func(Expr)) {
	if e == nil {
		return
	}
	fn(e)
	switch x := e.(type) {
	case *Unary:
		Walk(x.X, fn)
	case *Binary:
		Walk(x.L, fn)
		Walk(x.R, fn)
	case *Like:
		Walk(x.X, fn)
		Walk(x.Pattern, fn)
	case *Matches:
		Walk(x.X, fn)
		Walk(x.Pattern, fn)
	case *In:
		Walk(x.X, fn)
		for _, y := range x.List {
			Walk(y, fn)
		}
	case *Between:
		Walk(x.X, fn)
		Walk(x.Lo, fn)
		Walk(x.Hi, fn)
	case *IsNull:
		Walk(x.X, fn)
	case *Call:
		for _, y := range x.Args {
			Walk(y, fn)
		}
	}
}

// Fields returns the distinct field names an expression reads.
func Fields(exprs ...Expr) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range exprs {
		Walk(e, func(x Expr) {
			if f, ok := x.(*Field); ok && !seen[f.Name] {
				seen[f.Name] = true
				out = append(out, f.Name)
			}
		})
	}
	return out
}

func itoa(i int) string { return strconv.Itoa(i) }

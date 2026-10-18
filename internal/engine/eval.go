package engine

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

// row is anything an expression can read fields from.
type row interface {
	Get(field string) query.Value
}

// evaluator holds per-query state shared by all workers: the fixed value of
// NOW() and caches of compiled patterns.
type evaluator struct {
	now     time.Time
	likeRe  sync.Map // pattern -> *regexp.Regexp
	matchRe sync.Map // pattern -> *regexp.Regexp
}

func newEvaluator(now time.Time) *evaluator { return &evaluator{now: now} }

// scope is what an expression is evaluated against: a single entry, or a
// group of entries in an aggregate query.
type scope struct {
	row     row
	group   *group
	aliases map[string]query.Value // SELECT aliases, for ORDER BY
}

// RuntimeError is an error raised while evaluating an expression, such as
// adding a number to a name.
type RuntimeError struct {
	Expr query.Expr
	Msg  string
}

func (e *RuntimeError) Error() string { return e.Msg }

func (ev *evaluator) eval(e query.Expr, s scope) (query.Value, error) {
	if s.group != nil {
		if v, ok := s.group.keyValue(e.String()); ok {
			return v, nil
		}
	}

	switch x := e.(type) {
	case *query.Literal:
		return x.Value, nil

	case *query.Field:
		if x.Alias {
			return s.aliases[x.Name], nil
		}
		if s.row == nil {
			return query.Null, &RuntimeError{x, fmt.Sprintf("%s alanı bu bağlamda kullanılamaz", x.Name)}
		}
		return s.row.Get(x.Name), nil

	case *query.Unary:
		v, err := ev.eval(x.X, s)
		if err != nil {
			return query.Null, err
		}
		if x.Op == "NOT" {
			if v.IsNull() {
				return query.Null, nil
			}
			return query.Bool(!v.Truthy()), nil
		}
		return negate(x, v)

	case *query.Binary:
		return ev.binary(x, s)

	case *query.Like:
		return ev.like(x, s)

	case *query.Matches:
		return ev.matches(x, s)

	case *query.In:
		v, err := ev.eval(x.X, s)
		if err != nil || v.IsNull() {
			return query.Null, err
		}
		found := false
		for _, item := range x.List {
			w, err := ev.eval(item, s)
			if err != nil {
				return query.Null, err
			}
			if c, ok := query.Compare(v, w); ok && c == 0 {
				found = true
				break
			}
		}
		return query.Bool(found != x.Not), nil

	case *query.Between:
		v, err := ev.eval(x.X, s)
		if err != nil {
			return query.Null, err
		}
		lo, err := ev.eval(x.Lo, s)
		if err != nil {
			return query.Null, err
		}
		hi, err := ev.eval(x.Hi, s)
		if err != nil {
			return query.Null, err
		}
		c1, ok1 := query.Compare(v, lo)
		c2, ok2 := query.Compare(v, hi)
		if !ok1 || !ok2 {
			return query.Null, nil
		}
		return query.Bool((c1 >= 0 && c2 <= 0) != x.Not), nil

	case *query.IsNull:
		v, err := ev.eval(x.X, s)
		if err != nil {
			return query.Null, err
		}
		return query.Bool(v.IsNull() != x.Not), nil

	case *query.Call:
		if query.IsAggregateName(x.Name) {
			if s.group == nil {
				return query.Null, &RuntimeError{x, x.Name + " sadece toplama sorgularında kullanılabilir"}
			}
			return s.group.aggregate(x.String()), nil
		}
		return ev.call(x, s)
	}
	return query.Null, fmt.Errorf("desteklenmeyen ifade: %s", e)
}

func negate(e query.Expr, v query.Value) (query.Value, error) {
	switch v.Kind {
	case query.KindNull:
		return query.Null, nil
	case query.KindInt:
		return query.Int(-v.I), nil
	case query.KindFloat:
		return query.Float(-v.F), nil
	case query.KindDuration:
		return query.Value{Kind: query.KindDuration, I: -v.I}, nil
	}
	return query.Null, &RuntimeError{e, fmt.Sprintf("%s değeri negatif yapılamaz", v.Kind)}
}

func (ev *evaluator) binary(x *query.Binary, s scope) (query.Value, error) {
	l, err := ev.eval(x.L, s)
	if err != nil {
		return query.Null, err
	}

	// AND/OR use SQL three-valued logic and short-circuit.
	switch x.Op {
	case "AND":
		if !l.IsNull() && !l.Truthy() {
			return query.Bool(false), nil
		}
		r, err := ev.eval(x.R, s)
		if err != nil {
			return query.Null, err
		}
		if !r.IsNull() && !r.Truthy() {
			return query.Bool(false), nil
		}
		if l.IsNull() || r.IsNull() {
			return query.Null, nil
		}
		return query.Bool(true), nil
	case "OR":
		if !l.IsNull() && l.Truthy() {
			return query.Bool(true), nil
		}
		r, err := ev.eval(x.R, s)
		if err != nil {
			return query.Null, err
		}
		if !r.IsNull() && r.Truthy() {
			return query.Bool(true), nil
		}
		if l.IsNull() || r.IsNull() {
			return query.Null, nil
		}
		return query.Bool(false), nil
	}

	r, err := ev.eval(x.R, s)
	if err != nil {
		return query.Null, err
	}

	switch x.Op {
	case "+", "-":
		v, err := query.Arith(x.Op, l, r)
		if err != nil {
			return query.Null, &RuntimeError{x, err.Error()}
		}
		return v, nil
	}

	c, ok := query.Compare(l, r)
	if !ok {
		return query.Null, nil
	}
	switch x.Op {
	case "=":
		return query.Bool(c == 0), nil
	case "!=":
		return query.Bool(c != 0), nil
	case "<":
		return query.Bool(c < 0), nil
	case "<=":
		return query.Bool(c <= 0), nil
	case ">":
		return query.Bool(c > 0), nil
	case ">=":
		return query.Bool(c >= 0), nil
	}
	return query.Null, fmt.Errorf("bilinmeyen operatör %s", x.Op)
}

// likeRegexp turns a SQL LIKE pattern into a regular expression: % matches any
// run of characters and _ matches exactly one.
func likeRegexp(pattern string) *regexp.Regexp {
	var sb strings.Builder
	sb.WriteString("(?s)^")
	for _, r := range query.Fold(pattern) {
		switch r {
		case '%':
			sb.WriteString(".*")
		case '_':
			sb.WriteString(".")
		default:
			sb.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	sb.WriteString("$")
	return regexp.MustCompile(sb.String())
}

func (ev *evaluator) like(x *query.Like, s scope) (query.Value, error) {
	v, err := ev.eval(x.X, s)
	if err != nil {
		return query.Null, err
	}
	p, err := ev.eval(x.Pattern, s)
	if err != nil {
		return query.Null, err
	}
	if v.IsNull() || p.IsNull() {
		return query.Null, nil
	}
	if v.Kind != query.KindString || p.Kind != query.KindString {
		return query.Null, &RuntimeError{x, "LIKE sadece metinlerle kullanılabilir"}
	}
	re, ok := ev.likeRe.Load(p.S)
	if !ok {
		re, _ = ev.likeRe.LoadOrStore(p.S, likeRegexp(p.S))
	}
	return query.Bool(re.(*regexp.Regexp).MatchString(query.Fold(v.S)) != x.Not), nil
}

func (ev *evaluator) matches(x *query.Matches, s scope) (query.Value, error) {
	v, err := ev.eval(x.X, s)
	if err != nil {
		return query.Null, err
	}
	if v.IsNull() {
		return query.Null, nil
	}
	if v.Kind != query.KindString {
		return query.Null, &RuntimeError{x, "MATCHES sadece metinlerle kullanılabilir"}
	}
	re := x.Regex
	if re == nil {
		p, err := ev.eval(x.Pattern, s)
		if err != nil || p.IsNull() {
			return query.Null, err
		}
		cached, ok := ev.matchRe.Load(p.S)
		if !ok {
			compiled, err := regexp.Compile(p.S)
			if err != nil {
				return query.Null, &RuntimeError{x, "geçersiz düzenli ifade: " + err.Error()}
			}
			cached, _ = ev.matchRe.LoadOrStore(p.S, compiled)
		}
		re = cached.(*regexp.Regexp)
	}
	return query.Bool(re.MatchString(v.S) != x.Not), nil
}

func (ev *evaluator) call(x *query.Call, s scope) (query.Value, error) {
	switch x.Name {
	case "NOW":
		return query.Time(ev.now), nil
	case "TODAY":
		y, m, d := ev.now.Date()
		return query.Time(time.Date(y, m, d, 0, 0, 0, 0, ev.now.Location())), nil
	}

	arg, err := ev.eval(x.Args[0], s)
	if err != nil || arg.IsNull() {
		return query.Null, err
	}
	switch x.Name {
	case "LOWER", "UPPER", "LEN":
		if arg.Kind != query.KindString {
			return query.Null, &RuntimeError{x, x.Name + " bir metin bekler"}
		}
		switch x.Name {
		case "LOWER":
			return query.String(strings.ToLower(arg.S)), nil
		case "UPPER":
			return query.String(strings.ToUpper(arg.S)), nil
		default:
			return query.Int(int64(utf8.RuneCountInString(arg.S))), nil
		}
	case "YEAR", "MONTH", "DAY":
		if arg.Kind != query.KindTime {
			return query.Null, &RuntimeError{x, x.Name + " bir tarih bekler"}
		}
		switch x.Name {
		case "YEAR":
			return query.Int(int64(arg.T.Year())), nil
		case "MONTH":
			return query.Int(int64(arg.T.Month())), nil
		default:
			return query.Int(int64(arg.T.Day())), nil
		}
	}
	return query.Null, &RuntimeError{x, "bilinmeyen fonksiyon " + x.Name}
}

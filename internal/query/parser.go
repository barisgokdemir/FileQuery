package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Parse parses a single SELECT statement.
//
//	SELECT <* | expr [AS alias], ...>
//	FROM '<path>' [, '<path>' ...] [DEPTH n]
//	[WHERE expr]
//	[GROUP BY expr, ...]
//	[ORDER BY expr [ASC|DESC], ...]
//	[LIMIT n] [;]
func Parse(src string) (*Query, error) {
	toks, err := Lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks, src: src}
	q, err := p.parseQuery()
	if err != nil {
		return nil, err
	}
	if err := validate(q); err != nil {
		return nil, err
	}
	return q, nil
}

type parser struct {
	toks []Token
	i    int
	src  string

	aliases    map[string]bool // SELECT aliases, lower-case
	allowAlias bool            // true while parsing ORDER BY
}

func (p *parser) peek() Token { return p.toks[p.i] }
func (p *parser) next() Token {
	t := p.toks[p.i]
	if t.Kind != TokEOF {
		p.i++
	}
	return t
}

func (p *parser) isKeyword(words ...string) bool {
	t := p.peek()
	if t.Kind != TokKeyword {
		return false
	}
	for _, w := range words {
		if t.Text == w {
			return true
		}
	}
	return false
}

func (p *parser) acceptKeyword(w string) bool {
	if p.isKeyword(w) {
		p.i++
		return true
	}
	return false
}

func (p *parser) expectKeyword(w string) error {
	if !p.acceptKeyword(w) {
		return p.errorf(p.peek(), "%s bekleniyordu, %s bulundu", w, p.peek())
	}
	return nil
}

func (p *parser) accept(kind TokenKind) bool {
	if p.peek().Kind == kind {
		p.i++
		return true
	}
	return false
}

func (p *parser) errorf(t Token, format string, args ...any) error {
	return &Error{Pos: t.Pos, Msg: fmt.Sprintf(format, args...)}
}

func (p *parser) parseQuery() (*Query, error) {
	q := &Query{}
	if !p.isKeyword("SELECT") {
		return nil, p.errorf(p.peek(), "sorgu SELECT ile başlamalı (ör. SELECT name FROM 'C:\\' WHERE ext = 'pdf')")
	}
	p.next()

	if p.accept(TokStar) {
		q.Star = true
	} else {
		for {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			item := SelectItem{Expr: e}
			if p.acceptKeyword("AS") {
				t := p.next()
				if t.Kind != TokIdent && t.Kind != TokString {
					return nil, p.errorf(t, "AS sonrasında sütun adı bekleniyordu")
				}
				item.Alias = t.Text
			}
			q.Items = append(q.Items, item)
			if !p.accept(TokComma) {
				break
			}
		}
	}

	p.aliases = map[string]bool{}
	for _, it := range q.Items {
		if it.Alias != "" {
			p.aliases[strings.ToLower(it.Alias)] = true
		}
	}

	if err := p.expectKeyword("FROM"); err != nil {
		return nil, err
	}
	for {
		t := p.next()
		if t.Kind != TokString {
			return nil, p.errorf(t, "FROM sonrasında tırnak içinde bir klasör yolu bekleniyordu, ör. FROM 'C:\\Users'")
		}
		if strings.TrimSpace(t.Text) == "" {
			return nil, p.errorf(t, "klasör yolu boş olamaz")
		}
		q.Sources = append(q.Sources, t.Text)
		if !p.accept(TokComma) {
			break
		}
	}

	if p.acceptKeyword("DEPTH") {
		n, err := p.parsePositiveInt("DEPTH")
		if err != nil {
			return nil, err
		}
		q.Depth = n
	}

	if p.acceptKeyword("WHERE") {
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		q.Where = e
	}

	if p.acceptKeyword("GROUP") {
		if err := p.expectKeyword("BY"); err != nil {
			return nil, err
		}
		for {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			q.GroupBy = append(q.GroupBy, e)
			if !p.accept(TokComma) {
				break
			}
		}
	}

	if p.acceptKeyword("ORDER") {
		if err := p.expectKeyword("BY"); err != nil {
			return nil, err
		}
		p.allowAlias = true
		defer func() { p.allowAlias = false }()
		for {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			item := OrderItem{Expr: e}
			if p.acceptKeyword("DESC") {
				item.Desc = true
			} else {
				p.acceptKeyword("ASC")
			}
			q.OrderBy = append(q.OrderBy, item)
			if !p.accept(TokComma) {
				break
			}
		}
	}

	if p.acceptKeyword("LIMIT") {
		n, err := p.parsePositiveInt("LIMIT")
		if err != nil {
			return nil, err
		}
		q.Limit = n
	}

	p.accept(TokSemicolon)
	if t := p.peek(); t.Kind != TokEOF {
		return nil, p.errorf(t, "beklenmeyen %s; sorgu burada bitmeliydi", t)
	}
	return q, nil
}

func (p *parser) parsePositiveInt(clause string) (int, error) {
	t := p.next()
	if t.Kind != TokNumber || t.Suffix != "" || strings.Contains(t.Text, ".") {
		return 0, p.errorf(t, "%s sonrasında pozitif bir tam sayı bekleniyordu", clause)
	}
	n, err := strconv.Atoi(t.Text)
	if err != nil || n <= 0 {
		return 0, p.errorf(t, "%s sonrasında pozitif bir tam sayı bekleniyordu", clause)
	}
	return n, nil
}

// Expression grammar, lowest precedence first:
//
//	or        = and { OR and }
//	and       = not { AND not }
//	not       = NOT not | predicate
//	predicate = additive [ cmp additive | [NOT] LIKE additive | [NOT] MATCHES additive
//	                     | [NOT] IN ( list ) | [NOT] BETWEEN additive AND additive
//	                     | IS [NOT] NULL ]
//	additive  = unary { (+|-) unary }
//	unary     = - unary | primary
func (p *parser) parseExpr() (Expr, error) { return p.parseOr() }

func (p *parser) parseOr() (Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("OR") {
		t := p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: "OR", L: left, R: right, pos: t.Pos}
	}
	return left, nil
}

func (p *parser) parseAnd() (Expr, error) {
	left, err := p.parseNot()
	if err != nil {
		return nil, err
	}
	for p.isKeyword("AND") {
		t := p.next()
		right, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: "AND", L: left, R: right, pos: t.Pos}
	}
	return left, nil
}

func (p *parser) parseNot() (Expr, error) {
	if p.isKeyword("NOT") {
		t := p.next()
		x, err := p.parseNot()
		if err != nil {
			return nil, err
		}
		return &Unary{Op: "NOT", X: x, pos: t.Pos}, nil
	}
	return p.parsePredicate()
}

var compareOps = map[TokenKind]string{
	TokEq: "=", TokNeq: "!=", TokLt: "<", TokLte: "<=", TokGt: ">", TokGte: ">=",
}

func (p *parser) parsePredicate() (Expr, error) {
	left, err := p.parseAdditive()
	if err != nil {
		return nil, err
	}

	if op, ok := compareOps[p.peek().Kind]; ok {
		t := p.next()
		right, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		return &Binary{Op: op, L: left, R: right, pos: t.Pos}, nil
	}

	if p.isKeyword("IS") {
		t := p.next()
		negated := p.acceptKeyword("NOT")
		if err := p.expectKeyword("NULL"); err != nil {
			return nil, err
		}
		return &IsNull{X: left, Not: negated, pos: t.Pos}, nil
	}

	negated := false
	if p.isKeyword("NOT") {
		// Only consume NOT when it introduces LIKE/MATCHES/IN/BETWEEN.
		save := p.i
		p.next()
		if !p.isKeyword("LIKE", "MATCHES", "IN", "BETWEEN") {
			p.i = save
			return left, nil
		}
		negated = true
	}

	switch {
	case p.isKeyword("LIKE"):
		t := p.next()
		pat, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		return &Like{X: left, Pattern: pat, Not: negated, pos: t.Pos}, nil

	case p.isKeyword("MATCHES"):
		t := p.next()
		pat, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		m := &Matches{X: left, Pattern: pat, Not: negated, pos: t.Pos}
		if lit, ok := pat.(*Literal); ok && lit.Value.Kind == KindString {
			re, err := regexp.Compile(lit.Value.S)
			if err != nil {
				return nil, &Error{Pos: lit.pos, Msg: "geçersiz düzenli ifade: " + err.Error()}
			}
			m.Regex = re
		}
		return m, nil

	case p.isKeyword("IN"):
		t := p.next()
		// ext IN @resim
		if p.peek().Kind == TokSetRef {
			list, err := p.parseSetRef()
			if err != nil {
				return nil, err
			}
			return &In{X: left, List: list, Not: negated, pos: t.Pos}, nil
		}
		if !p.accept(TokLParen) {
			return nil, p.errorf(p.peek(), "IN sonrasında ( veya hazır liste bekleniyordu, ör. ext IN ('pdf', 'docx') ya da ext IN @resim")
		}
		var list []Expr
		for {
			// ext IN (@resim, @video, 'pdf')
			if p.peek().Kind == TokSetRef {
				items, err := p.parseSetRef()
				if err != nil {
					return nil, err
				}
				list = append(list, items...)
			} else {
				e, err := p.parseAdditive()
				if err != nil {
					return nil, err
				}
				list = append(list, e)
			}
			if !p.accept(TokComma) {
				break
			}
		}
		if !p.accept(TokRParen) {
			return nil, p.errorf(p.peek(), "IN listesini kapatan ) bekleniyordu")
		}
		return &In{X: left, List: list, Not: negated, pos: t.Pos}, nil

	case p.isKeyword("BETWEEN"):
		t := p.next()
		lo, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		if err := p.expectKeyword("AND"); err != nil {
			return nil, err
		}
		hi, err := p.parseAdditive()
		if err != nil {
			return nil, err
		}
		return &Between{X: left, Lo: lo, Hi: hi, Not: negated, pos: t.Pos}, nil
	}

	return left, nil
}

// parseSetRef expands @name into literal extensions.
func (p *parser) parseSetRef() ([]Expr, error) {
	t := p.next()
	set, ok := LookupExtSet(t.Text)
	if !ok {
		msg := fmt.Sprintf("bilinmeyen hazır liste @%s", t.Text)
		if s := suggest(t.Text, extSetNames()); s != "" {
			msg += fmt.Sprintf(" (@%s mi demek istediniz?)", s)
		} else {
			names := make([]string, len(ExtSets))
			for i, s := range ExtSets {
				names[i] = "@" + s.Name
			}
			msg += "; kullanılabilenler: " + strings.Join(names, ", ")
		}
		return nil, p.errorf(t, "%s", msg)
	}
	list := make([]Expr, len(set.Exts))
	for i, ext := range set.Exts {
		list[i] = &Literal{Value: String(ext), pos: t.Pos}
	}
	return list, nil
}

func (p *parser) parseAdditive() (Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.peek().Kind == TokPlus || p.peek().Kind == TokMinus {
		t := p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &Binary{Op: t.Text, L: left, R: right, pos: t.Pos}
	}
	return left, nil
}

func (p *parser) parseUnary() (Expr, error) {
	if p.peek().Kind == TokMinus {
		t := p.next()
		x, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		if lit, ok := x.(*Literal); ok {
			switch lit.Value.Kind {
			case KindInt:
				return &Literal{Value: Int(-lit.Value.I), pos: t.Pos, source: "-" + lit.String()}, nil
			case KindFloat:
				return &Literal{Value: Float(-lit.Value.F), pos: t.Pos, source: "-" + lit.String()}, nil
			case KindDuration:
				return &Literal{Value: Value{Kind: KindDuration, I: -lit.Value.I}, pos: t.Pos, source: "-" + lit.String()}, nil
			}
		}
		return &Unary{Op: "-", X: x, pos: t.Pos}, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (Expr, error) {
	t := p.next()
	switch t.Kind {
	case TokNumber:
		unit := t.Suffix
		source := t.Text + t.Suffix
		// Unit written as a separate word: 500 KB
		if unit == "" && p.peek().Kind == TokIdent && IsSizeUnit(p.peek().Text) {
			u := p.next()
			unit = u.Text
			source = t.Text + " " + u.Text
		}
		v, ok := numberLiteral(t.Text, unit)
		if !ok {
			return nil, p.errorf(t, "bilinmeyen birim %q; boyut için B, KB, MB, GB, TB; süre için s, m, h, d, w kullanın", unit)
		}
		return &Literal{Value: v, pos: t.Pos, source: source}, nil

	case TokString:
		return &Literal{Value: String(t.Text), pos: t.Pos}, nil

	case TokSetRef:
		return nil, p.errorf(t, "@%s gibi hazır listeler sadece IN ile kullanılabilir, ör. ext IN @%s", t.Text, t.Text)

	case TokKeyword:
		switch t.Text {
		case "TRUE":
			return &Literal{Value: Bool(true), pos: t.Pos}, nil
		case "FALSE":
			return &Literal{Value: Bool(false), pos: t.Pos}, nil
		case "NULL":
			return &Literal{Value: Null, pos: t.Pos}, nil
		case "GROUP", "DEPTH":
			// "group" and "depth" are also field names; inside an expression they
			// are never the GROUP BY / DEPTH clauses.
			return &Field{Name: strings.ToLower(t.Text), pos: t.Pos}, nil
		}
		return nil, p.errorf(t, "beklenmeyen %s; burada bir alan, değer veya ifade bekleniyordu", t)

	case TokLParen:
		e, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if !p.accept(TokRParen) {
			return nil, p.errorf(p.peek(), "kapatan ) bekleniyordu")
		}
		return e, nil

	case TokIdent:
		if p.peek().Kind == TokLParen {
			return p.parseCall(t)
		}
		if p.allowAlias && p.aliases[strings.ToLower(t.Text)] {
			return &Field{Name: strings.ToLower(t.Text), Alias: true, pos: t.Pos}, nil
		}
		if _, ok := LookupField(t.Text); !ok {
			msg := fmt.Sprintf("bilinmeyen alan %q", t.Text)
			if s := suggest(t.Text, fieldNames()); s != "" {
				msg += fmt.Sprintf(" (%s mi demek istediniz?)", s)
			}
			return nil, p.errorf(t, "%s", msg)
		}
		return &Field{Name: strings.ToLower(t.Text), pos: t.Pos}, nil
	}
	return nil, p.errorf(t, "beklenmeyen %s", t)
}

func (p *parser) parseCall(name Token) (Expr, error) {
	p.next() // (
	fn := strings.ToUpper(name.Text)
	call := &Call{Name: fn, pos: name.Pos}

	argc, isFunc := functions[fn]
	if !isFunc && !aggregates[fn] {
		msg := fmt.Sprintf("bilinmeyen fonksiyon %s", fn)
		if s := suggest(fn, FunctionNames()); s != "" {
			msg += fmt.Sprintf(" (%s mi demek istediniz?)", s)
		}
		return nil, p.errorf(name, "%s", msg)
	}

	if fn == "COUNT" && p.accept(TokStar) {
		call.Star = true
	} else if p.peek().Kind != TokRParen {
		for {
			e, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			call.Args = append(call.Args, e)
			if !p.accept(TokComma) {
				break
			}
		}
	}
	if !p.accept(TokRParen) {
		return nil, p.errorf(p.peek(), "%s( çağrısını kapatan ) bekleniyordu", fn)
	}

	want := argc
	if aggregates[fn] {
		want = 1
	}
	if !call.Star && len(call.Args) != want {
		return nil, p.errorf(name, "%s %d argüman alır, %d verildi", fn, want, len(call.Args))
	}
	if aggregates[fn] && !call.Star && IsAggregate(call.Args[0]) {
		return nil, p.errorf(name, "toplama fonksiyonları iç içe kullanılamaz")
	}
	return call, nil
}

// validate checks rules that need the whole query.
func validate(q *Query) error {
	if c := firstAggregate(q.Where); q.Where != nil && c != nil {
		return &Error{Pos: c.Pos(), Msg: "WHERE içinde COUNT/SUM gibi toplama fonksiyonları kullanılamaz"}
	}
	for _, g := range q.GroupBy {
		if c := firstAggregate(g); c != nil {
			return &Error{Pos: c.Pos(), Msg: "GROUP BY içinde toplama fonksiyonu kullanılamaz"}
		}
	}

	aggregate := len(q.GroupBy) > 0
	for _, it := range q.Items {
		if IsAggregate(it.Expr) {
			aggregate = true
		}
	}
	if !aggregate {
		for _, o := range q.OrderBy {
			if c := firstAggregate(o.Expr); c != nil {
				return &Error{Pos: c.Pos(), Msg: "ORDER BY'da toplama fonksiyonu için sorguda GROUP BY veya toplama sütunu olmalı"}
			}
		}
		return nil
	}

	if q.Star {
		return &Error{Pos: 0, Msg: "GROUP BY ile SELECT * kullanılamaz; sütunları tek tek yazın"}
	}
	grouped := map[string]bool{}
	for _, g := range q.GroupBy {
		grouped[g.String()] = true
	}
	for _, it := range q.Items {
		if err := checkGrouped(it.Expr, grouped); err != nil {
			return err
		}
	}
	for _, o := range q.OrderBy {
		if f, ok := o.Expr.(*Field); ok && f.Alias {
			continue
		}
		if err := checkGrouped(o.Expr, grouped); err != nil {
			return err
		}
	}
	return nil
}

// checkGrouped ensures that, outside aggregate calls, an expression only uses
// GROUP BY expressions.
func checkGrouped(e Expr, grouped map[string]bool) error {
	if grouped[e.String()] {
		return nil
	}
	switch x := e.(type) {
	case *Literal:
		return nil
	case *Call:
		if aggregates[x.Name] {
			return nil
		}
	case *Field:
		return &Error{Pos: x.pos, Msg: fmt.Sprintf("%s alanı GROUP BY'da yok; ya GROUP BY'a ekleyin ya da COUNT/SUM/MIN/MAX gibi bir fonksiyonla kullanın", x.Name)}
	}
	var err error
	Walk(e, func(sub Expr) {
		if err != nil || sub == e {
			return
		}
		if c, ok := sub.(*Call); ok && aggregates[c.Name] {
			return
		}
		if f, ok := sub.(*Field); ok && !grouped[f.String()] && !insideAggregate(e, f) {
			err = &Error{Pos: f.pos, Msg: fmt.Sprintf("%s alanı GROUP BY'da yok; ya GROUP BY'a ekleyin ya da COUNT/SUM/MIN/MAX gibi bir fonksiyonla kullanın", f.Name)}
		}
	})
	return err
}

// insideAggregate reports whether target appears inside an aggregate call in e.
func insideAggregate(e Expr, target Expr) bool {
	found := false
	Walk(e, func(sub Expr) {
		if c, ok := sub.(*Call); ok && aggregates[c.Name] {
			Walk(c, func(x Expr) {
				if x == target {
					found = true
				}
			})
		}
	})
	return found
}

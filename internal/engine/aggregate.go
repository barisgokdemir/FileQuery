package engine

import (
	"github.com/barisgokdemir/FileQuery/internal/query"
)

// accumulator computes one aggregate (COUNT, SUM, MIN, MAX, AVG) over a group.
type accumulator struct {
	call  *query.Call
	count int64
	sumI  int64
	sumF  float64
	float bool // switched to float arithmetic
	kind  query.Kind
	best  query.Value
}

func (a *accumulator) add(ev *evaluator, s scope) error {
	if a.call.Star {
		a.count++
		return nil
	}
	v, err := ev.eval(a.call.Args[0], s)
	if err != nil || v.IsNull() {
		return err
	}
	a.count++
	switch a.call.Name {
	case "SUM", "AVG":
		switch v.Kind {
		case query.KindInt, query.KindDuration:
			a.sumI += v.I
			a.sumF += float64(v.I)
		case query.KindFloat:
			a.float = true
			a.sumF += v.F
		default:
			return &RuntimeError{a.call, a.call.Name + " sadece sayı, boyut veya süreyle kullanılabilir"}
		}
		if a.kind == query.KindNull || v.Kind == query.KindFloat {
			a.kind = v.Kind
		}
	case "MIN", "MAX":
		if a.best.IsNull() {
			a.best = v
			return nil
		}
		c, ok := query.Compare(v, a.best)
		if ok && ((a.call.Name == "MIN" && c < 0) || (a.call.Name == "MAX" && c > 0)) {
			a.best = v
		}
	}
	return nil
}

func (a *accumulator) result() query.Value {
	switch a.call.Name {
	case "COUNT":
		return query.Int(a.count)
	case "SUM":
		if a.count == 0 {
			return query.Null
		}
		if a.float {
			return query.Float(a.sumF)
		}
		if a.kind == query.KindDuration {
			return query.Value{Kind: query.KindDuration, I: a.sumI}
		}
		return query.Int(a.sumI)
	case "AVG":
		if a.count == 0 {
			return query.Null
		}
		avg := a.sumF / float64(a.count)
		if a.kind == query.KindDuration {
			return query.Value{Kind: query.KindDuration, I: int64(avg)}
		}
		return query.Float(avg)
	default: // MIN, MAX
		return a.best
	}
}

// group is one GROUP BY bucket.
type group struct {
	keys  map[string]query.Value // GROUP BY expression text -> value
	accs  map[string]*accumulator
	order []string // aggregate keys in first-seen order
}

func newGroup(keys map[string]query.Value, calls []*query.Call) *group {
	g := &group{keys: keys, accs: map[string]*accumulator{}}
	for _, c := range calls {
		k := c.String()
		if _, ok := g.accs[k]; !ok {
			g.accs[k] = &accumulator{call: c}
			g.order = append(g.order, k)
		}
	}
	return g
}

func (g *group) keyValue(expr string) (query.Value, bool) {
	v, ok := g.keys[expr]
	return v, ok
}

func (g *group) aggregate(key string) query.Value {
	if a, ok := g.accs[key]; ok {
		return a.result()
	}
	return query.Null
}

func (g *group) add(ev *evaluator, s scope) error {
	for _, k := range g.order {
		if err := g.accs[k].add(ev, s); err != nil {
			return err
		}
	}
	return nil
}

// aggregateCalls collects the distinct aggregate calls in a set of expressions.
func aggregateCalls(exprs ...query.Expr) []*query.Call {
	seen := map[string]bool{}
	var out []*query.Call
	for _, e := range exprs {
		query.Walk(e, func(x query.Expr) {
			if c, ok := x.(*query.Call); ok && query.IsAggregateName(c.Name) && !seen[c.String()] {
				seen[c.String()] = true
				out = append(out, c)
			}
		})
	}
	return out
}

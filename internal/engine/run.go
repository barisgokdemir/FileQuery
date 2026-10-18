package engine

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/barisgokdemir/FileQuery/internal/query"
)

// Column describes one output column.
type Column struct {
	Name   string
	IsSize bool // values are byte counts; shown as KB/MB/GB in tables
}

// Stats describes a finished query.
type Stats struct {
	ScanStats
	Rows    int
	Elapsed time.Duration
}

// Options controls how results are produced.
type Options struct {
	// SortByPath orders rows by path when the query has no ORDER BY, so that
	// table output is stable. Rows are then buffered instead of streamed.
	SortByPath bool
}

// Run executes q and calls emit for every result row, in order. emit is never
// called concurrently.
func Run(ctx context.Context, q *query.Query, opts Options, emit func([]query.Value) error) (Stats, error) {
	start := time.Now()
	ev := newEvaluator(start)

	roots := make([]string, 0, len(q.Sources))
	for _, s := range q.Sources {
		r, err := ResolveRoot(s)
		if err != nil {
			return Stats{}, err
		}
		roots = append(roots, r)
	}

	items := selectItems(q)
	var (
		stats Stats
		err   error
	)
	if isAggregateQuery(q, items) {
		stats, err = runAggregate(ctx, q, items, roots, ev, emit)
	} else {
		stats, err = runPlain(ctx, q, items, roots, ev, opts, emit)
	}
	stats.Elapsed = time.Since(start)
	return stats, err
}

// Columns returns the output columns of q.
func Columns(q *query.Query) []Column {
	items := selectItems(q)
	cols := make([]Column, len(items))
	for i, it := range items {
		cols[i] = Column{Name: it.Name(), IsSize: isSizeExpr(it.Expr)}
	}
	return cols
}

func selectItems(q *query.Query) []query.SelectItem {
	if !q.Star {
		return q.Items
	}
	items := make([]query.SelectItem, len(query.DefaultColumns))
	for i, name := range query.DefaultColumns {
		f, _ := query.Parse("SELECT " + name + " FROM '.'")
		items[i] = f.Items[0]
	}
	return items
}

// isSizeExpr reports whether an expression yields a byte count.
func isSizeExpr(e query.Expr) bool {
	switch x := e.(type) {
	case *query.Field:
		return x.Name == "size"
	case *query.Call:
		switch x.Name {
		case "SUM", "MIN", "MAX", "AVG":
			return len(x.Args) == 1 && isSizeExpr(x.Args[0])
		}
	case *query.Binary:
		return (x.Op == "+" || x.Op == "-") && isSizeExpr(x.L) && isSizeExpr(x.R)
	}
	return false
}

func isAggregateQuery(q *query.Query, items []query.SelectItem) bool {
	if len(q.GroupBy) > 0 {
		return true
	}
	for _, it := range items {
		if query.IsAggregate(it.Expr) {
			return true
		}
	}
	return false
}

type resultRow struct {
	values []query.Value
	keys   []query.Value // ORDER BY keys
	path   string
}

// errorOnce keeps the first error raised by any worker.
type errorOnce struct {
	mu  sync.Mutex
	err error
}

func (e *errorOnce) set(err error) {
	e.mu.Lock()
	if e.err == nil {
		e.err = err
	}
	e.mu.Unlock()
}

func (e *errorOnce) get() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.err
}

func matchesWhere(ev *evaluator, where query.Expr, s scope) (bool, error) {
	if where == nil {
		return true, nil
	}
	v, err := ev.eval(where, s)
	if err != nil {
		return false, err
	}
	return v.Truthy(), nil
}

func runPlain(ctx context.Context, q *query.Query, items []query.SelectItem, roots []string,
	ev *evaluator, opts Options, emit func([]query.Value) error) (Stats, error) {

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream := len(q.OrderBy) == 0 && !opts.SortByPath
	var (
		mu      sync.Mutex
		rows    []resultRow
		emitted int
		failure errorOnce
	)

	scanStats := Scan(ctx, roots, q.Depth, func(e *Entry) bool {
		s := scope{row: e}
		ok, err := matchesWhere(ev, q.Where, s)
		if err != nil {
			failure.set(err)
			return false
		}
		if !ok {
			return true
		}

		values := make([]query.Value, len(items))
		var aliases map[string]query.Value
		for i, it := range items {
			v, err := ev.eval(it.Expr, s)
			if err != nil {
				failure.set(err)
				return false
			}
			values[i] = v
			if it.Alias != "" {
				if aliases == nil {
					aliases = map[string]query.Value{}
				}
				aliases[strings.ToLower(it.Alias)] = v
			}
		}

		var keys []query.Value
		if len(q.OrderBy) > 0 {
			s.aliases = aliases
			keys = make([]query.Value, len(q.OrderBy))
			for i, o := range q.OrderBy {
				v, err := ev.eval(o.Expr, s)
				if err != nil {
					failure.set(err)
					return false
				}
				keys[i] = v
			}
		}

		mu.Lock()
		defer mu.Unlock()
		if q.Limit > 0 && (emitted >= q.Limit || (!stream && len(q.OrderBy) == 0 && len(rows) >= q.Limit)) {
			return false
		}
		if stream {
			if err := emit(values); err != nil {
				failure.set(err)
				return false
			}
			emitted++
			return q.Limit == 0 || emitted < q.Limit
		}
		rows = append(rows, resultRow{values: values, keys: keys, path: e.Path})
		return true
	})

	stats := Stats{ScanStats: scanStats}
	if err := failure.get(); err != nil {
		return stats, err
	}
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return stats, err
	}
	if stream {
		stats.Rows = emitted
		return stats, nil
	}

	if len(q.OrderBy) > 0 {
		sortRows(rows, q.OrderBy)
	} else {
		sort.Slice(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	}
	if q.Limit > 0 && len(rows) > q.Limit {
		rows = rows[:q.Limit]
	}
	for _, r := range rows {
		if err := emit(r.values); err != nil {
			return stats, err
		}
	}
	stats.Rows = len(rows)
	return stats, nil
}

func runAggregate(ctx context.Context, q *query.Query, items []query.SelectItem, roots []string,
	ev *evaluator, emit func([]query.Value) error) (Stats, error) {

	exprs := make([]query.Expr, 0, len(items)+len(q.OrderBy))
	for _, it := range items {
		exprs = append(exprs, it.Expr)
	}
	for _, o := range q.OrderBy {
		exprs = append(exprs, o.Expr)
	}
	calls := aggregateCalls(exprs...)

	var (
		mu      sync.Mutex
		groups  = map[string]*group{}
		order   []string
		failure errorOnce
	)
	if len(q.GroupBy) == 0 {
		// Aggregates without GROUP BY always produce exactly one row.
		groups[""] = newGroup(map[string]query.Value{}, calls)
		order = append(order, "")
	}

	scanStats := Scan(ctx, roots, q.Depth, func(e *Entry) bool {
		s := scope{row: e}
		ok, err := matchesWhere(ev, q.Where, s)
		if err != nil {
			failure.set(err)
			return false
		}
		if !ok {
			return true
		}

		keyVals := map[string]query.Value{}
		var keyParts []string
		for _, g := range q.GroupBy {
			v, err := ev.eval(g, s)
			if err != nil {
				failure.set(err)
				return false
			}
			keyVals[g.String()] = v
			keyParts = append(keyParts, groupKey(v))
		}
		key := strings.Join(keyParts, "\x00")

		mu.Lock()
		defer mu.Unlock()
		grp, exists := groups[key]
		if !exists {
			grp = newGroup(keyVals, calls)
			groups[key] = grp
			order = append(order, key)
		}
		if err := grp.add(ev, s); err != nil {
			failure.set(err)
			return false
		}
		return true
	})

	stats := Stats{ScanStats: scanStats}
	if err := failure.get(); err != nil {
		return stats, err
	}

	rows := make([]resultRow, 0, len(groups))
	for _, key := range order {
		grp := groups[key]
		s := scope{group: grp, aliases: map[string]query.Value{}}
		values := make([]query.Value, len(items))
		for i, it := range items {
			v, err := ev.eval(it.Expr, s)
			if err != nil {
				return stats, err
			}
			values[i] = v
			if it.Alias != "" {
				s.aliases[strings.ToLower(it.Alias)] = v
			}
		}
		keys := make([]query.Value, len(q.OrderBy))
		for i, o := range q.OrderBy {
			v, err := ev.eval(o.Expr, s)
			if err != nil {
				return stats, err
			}
			keys[i] = v
		}
		rows = append(rows, resultRow{values: values, keys: keys, path: key})
	}

	if len(q.OrderBy) > 0 {
		sortRows(rows, q.OrderBy)
	} else {
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].path < rows[j].path })
	}
	if q.Limit > 0 && len(rows) > q.Limit {
		rows = rows[:q.Limit]
	}
	for _, r := range rows {
		if err := emit(r.values); err != nil {
			return stats, err
		}
	}
	stats.Rows = len(rows)
	return stats, nil
}

// groupKey turns a GROUP BY value into a map key. Text is folded so that
// "PDF" and "pdf" land in the same group, as they would on Windows.
func groupKey(v query.Value) string {
	if v.Kind == query.KindString {
		return "s:" + query.Fold(v.S)
	}
	return v.Kind.String() + ":" + v.String()
}

// sortRows orders rows by their ORDER BY keys. NULLs always sort last.
func sortRows(rows []resultRow, order []query.OrderItem) {
	sort.SliceStable(rows, func(i, j int) bool {
		for k, o := range order {
			a, b := rows[i].keys[k], rows[j].keys[k]
			switch {
			case a.IsNull() && b.IsNull():
				continue
			case a.IsNull():
				return false
			case b.IsNull():
				return true
			}
			c, ok := query.Compare(a, b)
			if !ok || c == 0 {
				continue
			}
			if o.Desc {
				return c > 0
			}
			return c < 0
		}
		return rows[i].path < rows[j].path
	})
}

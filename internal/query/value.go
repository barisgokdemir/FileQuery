package query

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

// Kind is the dynamic type of a Value.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	KindTime
	KindDuration
)

func (k Kind) String() string {
	return [...]string{"boş", "mantıksal", "tam sayı", "sayı", "metin", "tarih", "süre"}[k]
}

// Value is a dynamically typed value produced by evaluating an expression.
type Value struct {
	Kind Kind
	B    bool
	I    int64 // also bytes for sizes and nanoseconds for durations
	F    float64
	S    string
	T    time.Time
}

var Null = Value{}

func Bool(b bool) Value              { return Value{Kind: KindBool, B: b} }
func Int(i int64) Value              { return Value{Kind: KindInt, I: i} }
func Float(f float64) Value          { return Value{Kind: KindFloat, F: f} }
func String(s string) Value          { return Value{Kind: KindString, S: s} }
func Time(t time.Time) Value         { return Value{Kind: KindTime, T: t} }
func Duration(d time.Duration) Value { return Value{Kind: KindDuration, I: int64(d)} }

func (v Value) IsNull() bool { return v.Kind == KindNull }

// Truthy reports whether a value counts as true in a WHERE clause.
func (v Value) Truthy() bool {
	switch v.Kind {
	case KindBool:
		return v.B
	case KindInt, KindDuration:
		return v.I != 0
	case KindFloat:
		return v.F != 0
	case KindString:
		return v.S != ""
	case KindTime:
		return !v.T.IsZero()
	}
	return false
}

func (v Value) numeric() (float64, bool) {
	switch v.Kind {
	case KindInt:
		return float64(v.I), true
	case KindFloat:
		return v.F, true
	}
	return 0, false
}

// String renders the value for display in diagnostics.
func (v Value) String() string {
	switch v.Kind {
	case KindNull:
		return "NULL"
	case KindBool:
		if v.B {
			return "TRUE"
		}
		return "FALSE"
	case KindInt:
		return strconv.FormatInt(v.I, 10)
	case KindFloat:
		return strconv.FormatFloat(v.F, 'f', -1, 64)
	case KindString:
		return "'" + strings.ReplaceAll(v.S, "'", "''") + "'"
	case KindTime:
		return "'" + v.T.Format("2006-01-02 15:04:05") + "'"
	case KindDuration:
		return time.Duration(v.I).String()
	}
	return "?"
}

// Size units (binary, as Windows Explorer shows them).
var sizeUnits = map[string]int64{
	"B": 1, "KB": 1 << 10, "MB": 1 << 20, "GB": 1 << 30, "TB": 1 << 40,
}

// Duration suffixes for literals such as 7d or 12h.
var durationUnits = map[string]time.Duration{
	"s": time.Second, "sn": time.Second,
	"m": time.Minute, "dk": time.Minute,
	"h": time.Hour, "sa": time.Hour,
	"d": 24 * time.Hour, "g": 24 * time.Hour,
	"w": 7 * 24 * time.Hour, "hf": 7 * 24 * time.Hour,
}

// numberLiteral converts a number token with an optional unit into a Value:
// "2" -> Int, "1.5" -> Float, "2MB"/"2 MB" -> Int bytes, "7d" -> Duration.
func numberLiteral(text, unit string) (Value, bool) {
	f, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return Null, false
	}
	if unit == "" {
		if !strings.Contains(text, ".") && f <= math.MaxInt64 {
			i, err := strconv.ParseInt(text, 10, 64)
			if err == nil {
				return Int(i), true
			}
		}
		return Float(f), true
	}
	if mul, ok := sizeUnits[strings.ToUpper(unit)]; ok {
		return Int(int64(math.Round(f * float64(mul)))), true
	}
	if d, ok := durationUnits[strings.ToLower(unit)]; ok {
		return Duration(time.Duration(f * float64(d))), true
	}
	return Null, false
}

// IsSizeUnit reports whether word is a size unit (used when the unit is
// written as a separate word: 500 KB).
func IsSizeUnit(word string) bool {
	_, ok := sizeUnits[strings.ToUpper(word)]
	return ok
}

var timeLayouts = []string{
	"2006-01-02",
	"2006-01-02 15:04",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	time.RFC3339,
	"02.01.2006",
	"02.01.2006 15:04",
	"02.01.2006 15:04:05",
}

// ParseTime parses a date literal in the local time zone. Both ISO
// (2024-03-15) and Turkish (15.03.2024) day-first formats are accepted.
func ParseTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, layout := range timeLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// coerceTime turns a string into a time when it is compared with a time.
func coerceTime(v Value) (time.Time, bool) {
	switch v.Kind {
	case KindTime:
		return v.T, true
	case KindString:
		return ParseTime(v.S)
	}
	return time.Time{}, false
}

// Compare orders two values. ok is false when they cannot be compared (for
// example a size against a name, or anything against NULL); such comparisons
// are false in a WHERE clause, as in SQL.
//
// Text comparison is case-insensitive, matching how Windows treats file names.
func Compare(a, b Value) (cmp int, ok bool) {
	if a.IsNull() || b.IsNull() {
		return 0, false
	}
	if x, okx := a.numeric(); okx {
		if y, oky := b.numeric(); oky {
			return compareFloat(x, y), true
		}
	}
	if a.Kind == KindTime || b.Kind == KindTime {
		x, okx := coerceTime(a)
		y, oky := coerceTime(b)
		if !okx || !oky {
			return 0, false
		}
		return x.Compare(y), true
	}
	if a.Kind == KindDuration && b.Kind == KindDuration {
		return compareInt(a.I, b.I), true
	}
	if a.Kind == KindString && b.Kind == KindString {
		return strings.Compare(Fold(a.S), Fold(b.S)), true
	}
	if a.Kind == KindBool && b.Kind == KindBool {
		switch {
		case a.B == b.B:
			return 0, true
		case !a.B:
			return -1, true
		default:
			return 1, true
		}
	}
	return 0, false
}

// Fold lower-cases text for case-insensitive comparison. Turkish dotted and
// dotless I are folded to plain 'i' so that "İŞ" matches "iş" and "ış".
func Fold(s string) string {
	s = strings.NewReplacer("İ", "i", "I", "i", "ı", "i").Replace(s)
	return strings.ToLower(s)
}

func compareFloat(x, y float64) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

func compareInt(x, y int64) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// Arith applies + or - to two values.
func Arith(op string, a, b Value) (Value, error) {
	if a.IsNull() || b.IsNull() {
		return Null, nil
	}
	if a.Kind == KindTime || b.Kind == KindTime {
		switch {
		case a.Kind == KindTime && b.Kind == KindDuration && op == "+":
			return Time(a.T.Add(time.Duration(b.I))), nil
		case a.Kind == KindDuration && b.Kind == KindTime && op == "+":
			return Time(b.T.Add(time.Duration(a.I))), nil
		case a.Kind == KindTime && b.Kind == KindDuration && op == "-":
			return Time(a.T.Add(-time.Duration(b.I))), nil
		case op == "-":
			x, okx := coerceTime(a)
			y, oky := coerceTime(b)
			if okx && oky {
				return Duration(x.Sub(y)), nil
			}
		}
		return Null, fmt.Errorf("%s ile %s arasında %s yapılamaz", a.Kind, b.Kind, op)
	}
	if a.Kind == KindDuration && b.Kind == KindDuration {
		if op == "+" {
			return Duration(time.Duration(a.I + b.I)), nil
		}
		return Duration(time.Duration(a.I - b.I)), nil
	}
	if a.Kind == KindInt && b.Kind == KindInt {
		if op == "+" {
			return Int(a.I + b.I), nil
		}
		return Int(a.I - b.I), nil
	}
	x, okx := a.numeric()
	y, oky := b.numeric()
	if okx && oky {
		if op == "+" {
			return Float(x + y), nil
		}
		return Float(x - y), nil
	}
	return Null, fmt.Errorf("%s ile %s arasında %s yapılamaz", a.Kind, b.Kind, op)
}

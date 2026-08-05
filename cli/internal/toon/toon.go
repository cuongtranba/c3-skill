package toon

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var reNumber = regexp.MustCompile(`^-?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$`)

// stringerType lets nested struct fields that render themselves (e.g. time.Time,
// which implements fmt.Stringer) stay scalar instead of recursing into fields.
var stringerType = reflect.TypeOf((*fmt.Stringer)(nil)).Elem()

// enc carries the encoder mode so both output shapes share one traversal.
// multiline renders a multi-line string as a block scalar with real newlines;
// classic TOON escapes it onto a single physical line.
type enc struct{ multiline bool }

var (
	toonEnc = enc{}
	textEnc = enc{multiline: true}
)

// NeedsQuoting returns true if the string value needs TOON quoting.
func NeedsQuoting(s string) bool {
	if s == "" || s[0] == ' ' || s[len(s)-1] == ' ' {
		return true
	}
	if s == "true" || s == "false" || s == "null" {
		return true
	}
	if reNumber.MatchString(s) {
		return true
	}
	for _, c := range s {
		switch c {
		case ',', ':', '"', '\\', '[', ']', '{', '}', '\n', '\t', '\r':
			return true
		}
	}
	return false
}

// MarshalValue serializes a single value as a TOON string.
func MarshalValue(v any) string {
	if v == nil {
		return "null"
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.String:
		s := rv.String()
		if NeedsQuoting(s) {
			return quote(s)
		}
		return s
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", rv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", rv.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", rv.Float())
	case reflect.Bool:
		if rv.Bool() {
			return "true"
		}
		return "false"
	case reflect.Ptr, reflect.Interface:
		if rv.IsNil() {
			return "null"
		}
		return MarshalValue(rv.Elem().Interface())
	default:
		return fmt.Sprintf("%v", v)
	}
}

// blockIndent is how far a block scalar's lines sit inside their key. A fixed
// indent is what makes the block self-terminating: the first line that is not
// indented this far is the next field, not more content.
const blockIndent = "  "

// isBlockScalar reports whether e would render s as a block scalar rather than a
// quoted one-liner. CR is deliberately excluded — a bare \r inside a block would
// be invisible, so CRLF content keeps the escaped form.
func (e enc) isBlockScalar(s string) bool {
	return e.multiline && strings.Contains(s, "\n") && !strings.Contains(s, "\r")
}

// writeBlockScalar renders "<prefix> |<chomp>" followed by the value's real
// lines, each indented by blockIndent past indent. prefix is "key:" for a field
// or "-" for a list element.
func (e enc) writeBlockScalar(b *strings.Builder, indent, prefix, s string) {
	body, chomp := chompIndicator(s)
	fmt.Fprintf(b, "%s%s |%s\n", indent, prefix, chomp)
	inner := indent + blockIndent
	for _, line := range strings.Split(body, "\n") {
		if line == "" {
			// Blank lines stay blank so a block never carries trailing whitespace.
			b.WriteByte('\n')
			continue
		}
		b.WriteString(inner)
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

// chompIndicator splits a value into the lines to emit plus the YAML chomping
// indicator that makes its trailing newlines exactly recoverable: "-" strip
// (none), "" clip (exactly one), "+" keep (two or more).
func chompIndicator(s string) (body, indicator string) {
	if !strings.HasSuffix(s, "\n") {
		return s, "-"
	}
	body = strings.TrimSuffix(s, "\n")
	if strings.HasSuffix(body, "\n") {
		return body, "+"
	}
	return body, ""
}

func quote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return `"` + s + `"`
}

// MarshalTable serializes a slice of structs as a TOON tabular array.
// fields specifies which struct fields to include (by json tag name).
func MarshalTable(label string, items any, fields []string) (string, error) {
	return toonEnc.table(label, items, fields)
}

// MarshalTableText serializes a tabular array for --format text. Rows are
// positional and comma-delimited, so a block scalar has nowhere to live: cells
// keep the escaped TOON form and the grid is byte-identical to MarshalTable.
func MarshalTableText(label string, items any, fields []string) (string, error) {
	return toonEnc.table(label, items, fields)
}

func (e enc) table(label string, items any, fields []string) (string, error) {
	rv := reflect.ValueOf(items)
	if rv.Kind() != reflect.Slice {
		return "", fmt.Errorf("toon: items must be a slice, got %s", rv.Kind())
	}

	var b strings.Builder
	n := rv.Len()
	fmt.Fprintf(&b, "%s[%d]{%s}:\n", label, n, strings.Join(fields, ","))

	if n == 0 {
		return b.String(), nil
	}

	// Build field index map from json tags
	elemType := rv.Type().Elem()
	fieldIndices := resolveFieldIndices(elemType, fields)

	for i := 0; i < n; i++ {
		elem := rv.Index(i)
		b.WriteString("  ")
		for j, fi := range fieldIndices {
			if j > 0 {
				b.WriteByte(',')
			}
			if fi < 0 {
				// Field not found — empty
				continue
			}
			fv := elem.Field(fi)
			b.WriteString(toonEnc.fieldValue(fv))
		}
		b.WriteByte('\n')
	}

	return b.String(), nil
}

// MarshalObject serializes a struct or map as TOON key:value pairs.
func MarshalObject(v any) (string, error) { return toonEnc.object(v) }

// MarshalObjectText is MarshalObject for --format text: identical output except
// that multi-line string values become block scalars with real newlines.
func MarshalObjectText(v any) (string, error) { return textEnc.object(v) }

func (e enc) object(v any) (string, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "null\n", nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		return e.structWithIndent(rv, "")
	case reflect.Map:
		return e.mapValue(rv, "")
	default:
		return "", fmt.Errorf("toon: MarshalObject requires struct or map, got %s", rv.Kind())
	}
}

// MarshalAny serializes common command output shapes as TOON.
func MarshalAny(v any) (string, error) { return toonEnc.any(v) }

// MarshalAnyText is MarshalAny for --format text: multi-line string values
// become block scalars with real newlines.
func MarshalAnyText(v any) (string, error) { return textEnc.any(v) }

func (e enc) any(v any) (string, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return "null\n", nil
	}
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return "null\n", nil
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct, reflect.Map:
		return e.object(v)
	case reflect.Slice, reflect.Array:
		var b strings.Builder
		fmt.Fprintf(&b, "items[%d]:\n", rv.Len())
		out, err := e.sliceElements(rv, "  ")
		if err != nil {
			return "", err
		}
		b.WriteString(out)
		return b.String(), nil
	default:
		return MarshalValue(v) + "\n", nil
	}
}

func (e enc) structWithIndent(rv reflect.Value, indent string) (string, error) {
	var b strings.Builder
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if !sf.IsExported() {
			continue
		}
		tag := sf.Tag.Get("json")
		name, opts := parseTag(tag)
		if name == "-" {
			continue
		}
		if name == "" {
			name = sf.Name
		}

		fv := rv.Field(i)

		// Handle omitempty
		if strings.Contains(opts, "omitempty") && isZero(fv) {
			continue
		}

		// Handle map fields — render as nested
		if fv.Kind() == reflect.Map {
			fmt.Fprintf(&b, "%s%s:\n", indent, name)
			nested, err := e.mapValue(fv, indent+"  ")
			if err != nil {
				return "", err
			}
			b.WriteString(nested)
			continue
		}

		// Handle slices whose elements are structs/maps — render as a nested
		// indented block so the inner fields survive. Scalar slices fall through
		// to the flat comma-joined form below.
		if fv.Kind() == reflect.Slice && isStructLikeSlice(fv.Type()) {
			fmt.Fprintf(&b, "%s%s[%d]:\n", indent, name, fv.Len())
			nested, err := e.sliceElements(fv, indent+"  ")
			if err != nil {
				return "", err
			}
			b.WriteString(nested)
			continue
		}

		// Handle pointer fields
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				if strings.Contains(opts, "omitempty") {
					continue
				}
				fmt.Fprintf(&b, "%s%s: null\n", indent, name)
				continue
			}
			fv = fv.Elem()
		}

		// Handle a nested struct field (incl. pointer-to-struct deref'd above) by
		// recursing, so inner fields survive instead of collapsing to a %v dump.
		// Structs that render themselves (Stringer, e.g. time.Time) stay scalar.
		if fv.Kind() == reflect.Struct && !fv.Type().Implements(stringerType) {
			fmt.Fprintf(&b, "%s%s:\n", indent, name)
			nested, err := e.structWithIndent(fv, indent+"  ")
			if err != nil {
				return "", err
			}
			b.WriteString(nested)
			continue
		}

		if s, ok := e.blockString(fv); ok {
			e.writeBlockScalar(&b, indent, name+":", s)
			continue
		}

		fmt.Fprintf(&b, "%s%s: %s\n", indent, name, e.fieldValue(fv))
	}
	return b.String(), nil
}

func (e enc) mapValue(rv reflect.Value, indent string) (string, error) {
	var b strings.Builder
	keys := rv.MapKeys()
	// Sort string keys for deterministic output
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})
	for _, k := range keys {
		v := rv.MapIndex(k)
		if s, ok := e.blockString(v); ok {
			e.writeBlockScalar(&b, indent, k.String()+":", s)
			continue
		}
		fmt.Fprintf(&b, "%s%s: %s\n", indent, k.String(), e.fieldValue(v))
	}
	return b.String(), nil
}

// isStructLikeSlice reports whether a slice's elements are structs or maps
// (deref'ing one pointer level) — i.e. values that need a nested block rather
// than the flat comma-joined scalar form.
func isStructLikeSlice(t reflect.Type) bool {
	if t.Kind() != reflect.Slice {
		return false
	}
	et := t.Elem()
	if et.Kind() == reflect.Ptr {
		et = et.Elem()
	}
	if et.Implements(stringerType) || reflect.PointerTo(et).Implements(stringerType) {
		return false
	}
	return et.Kind() == reflect.Struct || et.Kind() == reflect.Map
}

// blockString unwraps a value to a plain string and reports whether this
// encoder should emit it as a block scalar instead of an inline key: value.
// Stringer-backed values (time.Time and friends) keep their scalar rendering.
func (e enc) blockString(v reflect.Value) (string, bool) {
	if !e.multiline {
		return "", false
	}
	for v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.String || v.Type() != reflect.TypeOf("") {
		return "", false
	}
	s := v.String()
	return s, e.isBlockScalar(s)
}

// sliceElements renders each element of a struct/map slice as an indented
// "-" block. Shared by MarshalAny (top-level slices) and nested struct fields.
func (e enc) sliceElements(rv reflect.Value, indent string) (string, error) {
	var b strings.Builder
	for i := 0; i < rv.Len(); i++ {
		elem := rv.Index(i)
		if elem.Kind() == reflect.Interface && !elem.IsNil() {
			elem = elem.Elem()
		}
		if elem.Kind() == reflect.Ptr {
			if elem.IsNil() {
				fmt.Fprintf(&b, "%s- null\n", indent)
				continue
			}
			elem = elem.Elem()
		}
		switch elem.Kind() {
		case reflect.Struct:
			fmt.Fprintf(&b, "%s-\n", indent)
			out, err := e.structWithIndent(elem, indent+"  ")
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		case reflect.Map:
			fmt.Fprintf(&b, "%s-\n", indent)
			out, err := e.mapValue(elem, indent+"  ")
			if err != nil {
				return "", err
			}
			b.WriteString(out)
		default:
			if s, ok := e.blockString(elem); ok {
				e.writeBlockScalar(&b, indent, "-", s)
				continue
			}
			fmt.Fprintf(&b, "%s- %s\n", indent, e.fieldValue(elem))
		}
	}
	return b.String(), nil
}

func (e enc) fieldValue(fv reflect.Value) string {
	if fv.Kind() == reflect.Interface {
		if fv.IsNil() {
			return "null"
		}
		fv = fv.Elem()
	}
	if fv.Kind() == reflect.Ptr && fv.IsNil() {
		return "null"
	}
	if fv.CanInterface() && fv.Type().Implements(stringerType) {
		return MarshalValue(fv.Interface().(fmt.Stringer).String())
	}
	if fv.CanAddr() && fv.Addr().CanInterface() && fv.Addr().Type().Implements(stringerType) {
		return MarshalValue(fv.Addr().Interface().(fmt.Stringer).String())
	}

	switch fv.Kind() {
	case reflect.String:
		s := fv.String()
		if s == "" {
			return ""
		}
		if NeedsQuoting(s) {
			return quote(s)
		}
		return s
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fmt.Sprintf("%d", fv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fmt.Sprintf("%d", fv.Uint())
	case reflect.Float32, reflect.Float64:
		return fmt.Sprintf("%g", fv.Float())
	case reflect.Bool:
		if fv.Bool() {
			return "true"
		}
		return "false"
	case reflect.Ptr:
		if fv.IsNil() {
			return "null"
		}
		return e.fieldValue(fv.Elem())
	case reflect.Slice:
		if fv.IsNil() || fv.Len() == 0 {
			return ""
		}
		var parts []string
		for i := 0; i < fv.Len(); i++ {
			parts = append(parts, e.fieldValue(fv.Index(i)))
		}
		return strings.Join(parts, ",")
	default:
		return fmt.Sprintf("%v", fv.Interface())
	}
}

func resolveFieldIndices(t reflect.Type, fields []string) []int {
	tagMap := make(map[string]int)
	for i := 0; i < t.NumField(); i++ {
		sf := t.Field(i)
		tag := sf.Tag.Get("json")
		name, _ := parseTag(tag)
		if name == "" {
			name = sf.Name
		}
		tagMap[name] = i
	}

	indices := make([]int, len(fields))
	for i, f := range fields {
		if idx, ok := tagMap[f]; ok {
			indices[i] = idx
		} else {
			indices[i] = -1
		}
	}
	return indices
}

func parseTag(tag string) (string, string) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tag[idx+1:]
	}
	return tag, ""
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.IsNil() || v.Len() == 0
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	default:
		return false
	}
}

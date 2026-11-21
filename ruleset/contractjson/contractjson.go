// Package contractjson decodes the JSON documents of the engine contract:
// rule set snapshots and engine inputs (SAD §5.1, §5.2).
//
// The decoders and contracts/ruleset.schema.json agree on shape: member
// names, required members, JSON types, no null, and the syntax of the values
// Go parses itself (Span, Date). Every content rule belongs to
// ruleset.Validate and engine.Validate, and so does every schema keyword
// this package does not enforce: enums, minLength, minItems, uniqueItems and
// the Id pattern.
//
// Documents are I-JSON texts (RFC 7493), which a schema cannot express: a
// duplicate member name, an unpaired surrogate escape or invalid UTF-8 is a
// decode error, never a guess at which duplicate wins or a U+FFFD.
//
// The rules come from the struct tags, so a slice that adds a member needs
// no change here:
//   - null is never valid; an optional member is omitted;
//   - a member whose field has neither omitempty nor omitzero is required.
package contractjson

import (
	"encoding"
	"encoding/json/v2"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// Kind is the kind of a shape Error.
type Kind string

// Shape error kinds. The text is part of the error message.
const (
	KindNull    Kind = "null is not allowed"
	KindMissing Kind = "missing required member"
)

// Error is a shape violation at Pointer, a JSON pointer (RFC 6901) into the
// document in the same form as a Problem path.
type Error struct {
	Pointer string
	Kind    Kind
}

func (e *Error) Error() string { return fmt.Sprintf("%s at %q", e.Kind, e.Pointer) }

// Unmarshal decodes b into v (non-nil pointer to struct): json/v2 with
// RejectUnknownMembers, then a shape pass that rejects JSON null anywhere and
// any missing member whose field has neither omitempty nor omitzero.
// Types that decode themselves (encoding.TextUnmarshaler such as Span and
// Date, json.Unmarshaler such as jsontext.Value) are leaves; pointers are
// dereferenced; slices and map values are walked.
func Unmarshal(b []byte, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() || rv.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("contractjson: Unmarshal needs a non-nil pointer to a struct, got %T", v)
	}
	if err := decode(b, v, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	// The first pass has checked syntax, duplicate members and types, so the
	// generic tree has the shape of v wherever the shape pass looks.
	var tree any
	if err := decode(b, &tree); err != nil {
		return err
	}
	return shape(rv.Elem().Type(), tree, "")
}

// decode is the package's only call of json.Unmarshal.
func decode(b []byte, v any, opts ...json.Options) error {
	return json.Unmarshal(b, v, opts...) //nolint:forbidigo // the one place contract JSON is decoded; it adds the shape pass
}

var (
	textUnmarshaler = reflect.TypeFor[encoding.TextUnmarshaler]()
	jsonUnmarshaler = reflect.TypeFor[json.Unmarshaler]()
	jsonFrom        = reflect.TypeFor[json.UnmarshalerFrom]()
)

// leaf reports whether values of t decode themselves, so their content is
// not walked: text types (Span, Date) and raw JSON (jsontext.Value).
func leaf(t reflect.Type) bool {
	p := reflect.PointerTo(t)
	return p.Implements(textUnmarshaler) || p.Implements(jsonUnmarshaler) || p.Implements(jsonFrom)
}

func shape(t reflect.Type, node any, ptr string) error {
	if node == nil {
		return &Error{Pointer: ptr, Kind: KindNull}
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if leaf(t) {
		return nil
	}
	switch t.Kind() {
	case reflect.Struct:
		obj, _ := node.(map[string]any)
		for _, m := range Members(t) {
			child, ok := obj[m.Name]
			if !ok {
				if m.Required {
					return &Error{Pointer: ptr + "/" + escape(m.Name), Kind: KindMissing}
				}
				continue
			}
			if err := shape(m.Type, child, ptr+"/"+escape(m.Name)); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		arr, _ := node.([]any)
		for i, e := range arr {
			if err := shape(t.Elem(), e, ptr+"/"+strconv.Itoa(i)); err != nil {
				return err
			}
		}
	case reflect.Map:
		obj, _ := node.(map[string]any)
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if err := shape(t.Elem(), obj[k], ptr+"/"+escape(k)); err != nil {
				return err
			}
		}
	}
	return nil
}

// escape escapes one reference token of a JSON pointer (RFC 6901 §3).
func escape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// Member is one JSON member of a contract struct.
type Member struct {
	Name     string
	Type     reflect.Type
	Required bool // the field has neither omitempty nor omitzero
}

// Members returns the JSON members of struct type t in field order,
// skipping unexported fields and fields tagged "-". It is the one definition
// of a required member, shared by the decoders and the schema test. It
// panics on an embedded field and on the inline and unknown options: the
// contract structs have none, and json/v2 would merge their members into the
// parent object.
func Members(t reflect.Type) []Member {
	var out []Member
	for f := range t.Fields() {
		if f.Anonymous {
			panic(errors.New("contractjson: embedded field " + f.Name + " in " + t.String()))
		}
		if !f.IsExported() {
			continue
		}
		name, opts, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "-" && opts == "" {
			continue
		}
		if name == "" {
			name = f.Name
		}
		required := true
		for o := range strings.SplitSeq(opts, ",") {
			switch o {
			case "omitempty", "omitzero":
				required = false
			case "inline", "unknown":
				panic(errors.New("contractjson: option " + o + " on " + t.String() + "." + f.Name))
			}
		}
		out = append(out, Member{Name: name, Type: f.Type, Required: required})
	}
	return out
}

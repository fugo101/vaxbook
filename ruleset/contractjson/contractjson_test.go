package contractjson_test

import (
	"encoding/json/jsontext"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

// word is a leaf type with its own text form, like ruleset.Span.
type word string

func (w *word) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		return errors.New("empty word")
	}
	*w = word(b)
	return nil
}

type inner struct {
	Name  string `json:"name"`
	Note  string `json:"note,omitempty"`
	Count int    `json:"count,omitzero"`
}

type outer struct {
	ID     string           `json:"id"`
	Word   word             `json:"word"`
	Opt    *word            `json:"opt,omitzero"`
	Inner  inner            `json:"inner"`
	Ptr    *inner           `json:"ptr,omitzero"`
	List   []inner          `json:"list"`
	Tags   []string         `json:"tags,omitempty"`
	ByName map[string]inner `json:"byName,omitzero"`
	Skip   string           `json:"-"`
}

const valid = `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": [{"name": "x"}]}`

func TestUnmarshalAcceptsValid(t *testing.T) {
	t.Parallel()
	var v outer
	if err := contractjson.Unmarshal([]byte(valid), &v); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := outer{ID: "a", Word: "w", Inner: inner{Name: "n"}, List: []inner{{Name: "x"}}}
	if !reflect.DeepEqual(v, want) {
		t.Errorf("Unmarshal = %+v, want %+v", v, want)
	}
	// Optional members may be present too.
	full := `{"id": "a", "word": "w", "opt": "o", "inner": {"name": "n", "note": "x", "count": 2},
	  "ptr": {"name": "p"}, "list": [], "tags": ["t"], "byName": {"k": {"name": "m"}}}`
	if err := contractjson.Unmarshal([]byte(full), &v); err != nil {
		t.Errorf("Unmarshal with optional members: %v", err)
	}
}

func TestUnmarshalShapeErrors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, doc string
		kind      contractjson.Kind
		pointer   string
	}{
		{"null root", `null`, contractjson.KindNull, ""},
		{"missing member", `{"word": "w", "inner": {"name": "n"}, "list": []}`, contractjson.KindMissing, "/id"},
		{"missing nested member", `{"id": "a", "word": "w", "inner": {}, "list": []}`, contractjson.KindMissing, "/inner/name"},
		{"missing member in array element", `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": [{"name": "x"}, {}]}`, contractjson.KindMissing, "/list/1/name"},
		{"missing member behind pointer", `{"id": "a", "word": "w", "inner": {"name": "n"}, "ptr": {}, "list": []}`, contractjson.KindMissing, "/ptr/name"},
		{"missing member in map value", `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": [], "byName": {"a/b~": {}}}`, contractjson.KindMissing, "/byName/a~1b~0/name"},
		{"null member", `{"id": null, "word": "w", "inner": {"name": "n"}, "list": []}`, contractjson.KindNull, "/id"},
		{"null text leaf", `{"id": "a", "word": null, "inner": {"name": "n"}, "list": []}`, contractjson.KindNull, "/word"},
		{"null optional member", `{"id": "a", "word": "w", "opt": null, "inner": {"name": "n"}, "list": []}`, contractjson.KindNull, "/opt"},
		{"null nested optional", `{"id": "a", "word": "w", "inner": {"name": "n", "note": null}, "list": []}`, contractjson.KindNull, "/inner/note"},
		{"null array", `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": null}`, contractjson.KindNull, "/list"},
		{"null array element", `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": [{"name": "x"}, null]}`, contractjson.KindNull, "/list/1"},
		{"null string element", `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": [], "tags": ["t", null]}`, contractjson.KindNull, "/tags/1"},
		{"null object", `{"id": "a", "word": "w", "inner": null, "list": []}`, contractjson.KindNull, "/inner"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			var v outer
			err := contractjson.Unmarshal([]byte(c.doc), &v)
			var se *contractjson.Error
			if !errors.As(err, &se) {
				t.Fatalf("Unmarshal error = %v, want *contractjson.Error", err)
			}
			if se.Kind != c.kind || se.Pointer != c.pointer {
				t.Errorf("error = {%q %q}, want {%q %q}", se.Kind, se.Pointer, c.kind, c.pointer)
			}
		})
	}
}

func TestErrorMessage(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		err  contractjson.Error
		want string
	}{
		{contractjson.Error{Pointer: "/series", Kind: contractjson.KindMissing}, `missing required member at "/series"`},
		{contractjson.Error{Pointer: "/doses/0/date/value", Kind: contractjson.KindNull}, `null is not allowed at "/doses/0/date/value"`},
	} {
		if got := c.err.Error(); got != c.want {
			t.Errorf("Error() = %q, want %q", got, c.want)
		}
	}
}

func TestUnmarshalSyntaxErrors(t *testing.T) {
	t.Parallel()
	for name, doc := range map[string]string{
		"unknown member":   strings.Replace(valid, `"id"`, `"zz": 1, "id"`, 1),
		"wrong case":       strings.Replace(valid, `"id"`, `"ID"`, 1),
		"duplicate member": strings.Replace(valid, `"id": "a"`, `"id": "a", "id": "b"`, 1),
		"wrong type":       strings.Replace(valid, `"id": "a"`, `"id": 1`, 1),
		"bad text leaf":    strings.Replace(valid, `"word": "w"`, `"word": ""`, 1),
		"trailing data":    valid + `{}`,
		"not JSON":         `{`,
	} {
		var v outer
		err := contractjson.Unmarshal([]byte(doc), &v)
		var se *contractjson.Error
		if err == nil || errors.As(err, &se) {
			t.Errorf("%s: Unmarshal error = %v, want a JSON decode error", name, err)
		}
	}
}

func TestUnmarshalNeedsStructPointer(t *testing.T) {
	t.Parallel()
	var s string
	if err := contractjson.Unmarshal([]byte(`"x"`), &s); err == nil {
		t.Error("Unmarshal into *string succeeded")
	}
	if err := contractjson.Unmarshal([]byte(valid), outer{}); err == nil {
		t.Error("Unmarshal into a non-pointer succeeded")
	}
}

func TestMembers(t *testing.T) {
	t.Parallel()
	got := contractjson.Members(reflect.TypeFor[outer]())
	want := []contractjson.Member{
		{Name: "id", Type: reflect.TypeFor[string](), Required: true},
		{Name: "word", Type: reflect.TypeFor[word](), Required: true},
		{Name: "opt", Type: reflect.TypeFor[*word]()},
		{Name: "inner", Type: reflect.TypeFor[inner](), Required: true},
		{Name: "ptr", Type: reflect.TypeFor[*inner]()},
		{Name: "list", Type: reflect.TypeFor[[]inner](), Required: true},
		{Name: "tags", Type: reflect.TypeFor[[]string]()},
		{Name: "byName", Type: reflect.TypeFor[map[string]inner]()},
	}
	if !slices.Equal(got, want) {
		t.Errorf("Members =\n  %+v\nwant\n  %+v", got, want)
	}
	type untagged struct{ Plain string }
	if got := contractjson.Members(reflect.TypeFor[untagged]()); len(got) != 1 || got[0].Name != "Plain" || !got[0].Required {
		t.Errorf("Members(untagged) = %+v", got)
	}
}

// A member that decodes itself (jsontext.Value, a TextUnmarshaler) is a
// leaf: its content is not walked, but the member itself may not be null.
func TestUnmarshalRawLeaf(t *testing.T) {
	t.Parallel()
	type withRaw struct {
		Raw jsontext.Value `json:"raw"`
	}
	var v withRaw
	if err := contractjson.Unmarshal([]byte(`{"raw": [null, {"a": null}]}`), &v); err != nil {
		t.Errorf("raw content: %v", err)
	}
	var se *contractjson.Error
	if err := contractjson.Unmarshal([]byte(`{"raw": null}`), &v); !errors.As(err, &se) || se.Kind != contractjson.KindNull {
		t.Errorf("null raw member: %v, want a null error", err)
	}
}

func TestMembersPanicsOnUnsupportedOption(t *testing.T) {
	t.Parallel()
	type inlined struct {
		Rest map[string]string `json:",inline"`
	}
	defer func() {
		if recover() == nil {
			t.Error("Members did not panic on an inline member")
		}
	}()
	contractjson.Members(reflect.TypeFor[inlined]())
}

func TestMembersPanicsOnEmbeddedField(t *testing.T) {
	t.Parallel()
	type embedded struct {
		inner
		ID string `json:"id"`
	}
	defer func() {
		if recover() == nil {
			t.Error("Members did not panic on an embedded field")
		}
	}()
	contractjson.Members(reflect.TypeFor[embedded]())
}

// Contract texts are I-JSON (RFC 7493): a JSON Schema cannot express unique
// member names or well-formed Unicode, so the decoders reject such texts as a
// decode error instead of guessing which duplicate wins or replacing
// characters with U+FFFD. The schema description and SAD §5.1 say so.
func TestUnmarshalRejectsNonIJSON(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"duplicate member":          `{"id": "a", "id": "b", "word": "w", "inner": {"name": "n"}, "list": []}`,
		"duplicate nested member":   `{"id": "a", "word": "w", "inner": {"name": "n", "name": "m"}, "list": []}`,
		"lone high surrogate":       `{"id": "\ud83d", "word": "w", "inner": {"name": "n"}, "list": []}`,
		"lone low surrogate":        `{"id": "a\ude00", "word": "w", "inner": {"name": "n"}, "list": []}`,
		"invalid UTF-8":             "{\"id\": \"\xff\", \"word\": \"w\", \"inner\": {\"name\": \"n\"}, \"list\": []}",
		"lone surrogate in a value": `{"id": "a", "word": "w", "inner": {"name": "n"}, "list": [], "tags": ["\udfff"]}`,
	}
	for name, doc := range cases {
		var v outer
		if err := contractjson.Unmarshal([]byte(doc), &v); err == nil {
			t.Errorf("%s: Unmarshal = nil, want an error", name)
		}
	}
	// A surrogate pair is a well-formed character and decodes.
	var v outer
	if err := contractjson.Unmarshal([]byte(`{"id": "😀", "word": "w", "inner": {"name": "n"}, "list": []}`), &v); err != nil || v.ID != "😀" {
		t.Errorf("surrogate pair: Unmarshal = %v, id %q; want nil, 😀", err, v.ID)
	}
}

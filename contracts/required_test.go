package contracts_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fugo101/vaxbook/engine"
	"github.com/fugo101/vaxbook/ruleset"
	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

// s1Required lists, as JSON pointer patterns ("*" for any array index or map
// key), every required member of the stored contract documents at the end of
// engine slice S1: the rule set snapshot and the engine Input.
//
// R5 (SAD §5.1, CODING_STANDARDS.md): a member added after S1 is optional
// (omitzero) unless every stored document is migrated. Under R4 a field
// without omitempty or omitzero is required in the decoders and the schema
// alike, and TestSchemaMatchesStructs only checks that the two agree, so this
// frozen list is what makes a new required member a reviewed decision.
var s1Required = map[string][]string{
	"snapshot": {
		"/diseases",
		"/diseases/*/id",
		"/diseases/*/name",
		"/series",
		"/series/*/disease",
		"/series/*/id",
		"/series/*/plans",
		"/series/*/plans/*/doses",
		"/series/*/plans/*/doses/*/recommended",
		"/series/*/plans/*/doses/*/recommended/from",
		"/series/*/plans/*/doses/*/recommended/offset",
		"/series/*/plans/*/id",
		"/series/*/stage",
		"/version",
	},
	"input": {
		"/doses",
		"/doses/*/date",
		"/doses/*/date/precision",
		"/doses/*/diseases",
		"/doses/*/id",
		"/profile",
		"/profile/birth",
		"/profile/birth/precision",
		"/today",
	},
}

func TestRequiredMembersAreFrozen(t *testing.T) {
	t.Parallel()
	roots := map[string]reflect.Type{
		"snapshot": reflect.TypeFor[ruleset.Snapshot](),
		"input":    reflect.TypeFor[engine.Input](),
	}
	for name, typ := range roots {
		var got []string
		requiredMembers(typ, "", &got)
		slices.Sort(got)
		want := slices.Sorted(slices.Values(s1Required[name]))
		for _, p := range got {
			if !slices.Contains(want, p) {
				t.Errorf("%s: %s is a new required member. Stored %s documents without it no longer decode "+
					"(for example a rollback that copies an older snapshot, or an Input from an older guest client). Tag the field omitzero (R5), or "+
					"migrate every stored document and then add it to s1Required. A required member inside an "+
					"object that older documents never have is safe: add it to s1Required.", name, p, name)
			}
		}
		for _, p := range want {
			if !slices.Contains(got, p) {
				t.Errorf("%s: %s is in s1Required but no longer required; remove it from the list", name, p)
			}
		}
	}
}

// requiredMembers appends the pointer pattern of every required member
// reachable from t. It walks the types the way contractjson's shape pass
// walks documents: pointers are dereferenced, slices and map values are
// walked, and types that decode themselves (Span, Date) are leaves.
func requiredMembers(t reflect.Type, ptr string, out *[]string) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if p := reflect.PointerTo(t); p.Implements(reflect.TypeFor[interface{ UnmarshalText([]byte) error }]()) {
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		for _, m := range contractjson.Members(t) {
			p := ptr + "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(m.Name)
			if m.Required {
				*out = append(*out, p)
			}
			requiredMembers(m.Type, p, out)
		}
	case reflect.Slice, reflect.Array, reflect.Map:
		requiredMembers(t.Elem(), ptr+"/*", out)
	}
}

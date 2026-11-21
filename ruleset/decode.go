package ruleset

import (
	"fmt"

	"github.com/fugo101/vaxbook/ruleset/contractjson"
)

// Decode parses a snapshot strictly, with the shape rules of
// contracts/ruleset.schema.json: unknown or duplicate members, members in
// the wrong case, missing required members, JSON null and malformed spans
// are errors. It does not run Validate.
func Decode(b []byte) (*Snapshot, error) {
	var s Snapshot
	if err := contractjson.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("decode rule set snapshot: %w", err)
	}
	return &s, nil
}

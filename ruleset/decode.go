package ruleset

import (
	"encoding/json/v2"
	"fmt"
)

// Decode parses a snapshot strictly: unknown or duplicate members, members
// in the wrong case and malformed spans are errors. It does not run
// Validate.
func Decode(b []byte) (*Snapshot, error) {
	var s Snapshot
	if err := json.Unmarshal(b, &s, json.RejectUnknownMembers(true)); err != nil {
		return nil, fmt.Errorf("decode rule set snapshot: %w", err)
	}
	return &s, nil
}

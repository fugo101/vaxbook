// Package ent holds the ent schema (./schema) and the code generated from it.
// Regenerate with `task gen`; then run `task migrate:diff NAME=<change>` to write
// the matching goose migration (ADR-0009).
package ent

//go:generate go tool -modfile=../../../../../tools/go.mod ent generate --feature sql/versioned-migration,sql/lock,sql/execquery ./schema

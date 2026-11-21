package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Account is the login identity, identified by email only (GLOSSARY: Account).
// It holds no medical data.
type Account struct{ ent.Schema }

func (Account) Mixin() []ent.Mixin {
	return []ent.Mixin{idMixin{}, createdAtMixin{}}
}

func (Account) Fields() []ent.Field {
	return []ent.Field{
		// Stored already normalised (trimmed, lowercased); citext keeps the unique
		// constraint case-insensitive even if a caller forgets to normalise.
		field.String("email").
			SchemaType(map[string]string{dialect.Postgres: "citext"}).
			NotEmpty().
			Unique(),
	}
}

func (Account) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("sessions", Session.Type).
			Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

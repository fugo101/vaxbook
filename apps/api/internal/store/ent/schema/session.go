package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/google/uuid"
)

// Session is a server-side login session (ADR-0010). The primary key is the
// hex-encoded SHA-256 hash of the cookie token, so a database leak does not leak
// usable cookies.
type Session struct{ ent.Schema }

func (Session) Mixin() []ent.Mixin {
	return []ent.Mixin{createdAtMixin{}}
}

func (Session) Fields() []ent.Field {
	return []ent.Field{
		field.String("id").MinLen(64).MaxLen(64).Immutable(),
		field.UUID("account_id", uuid.UUID{}).Immutable(),
		field.Time("last_seen_at").Default(time.Now),
		field.Time("expires_at"),
	}
}

func (Session) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("account", Account.Type).
			Ref("sessions").
			Field("account_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (Session) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("expires_at"),
	}
}

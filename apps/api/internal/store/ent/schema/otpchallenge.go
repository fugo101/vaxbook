package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// OtpChallenge is a pending email sign-in code (SAD §6.1). It stores no plaintext
// email: the email and the code are kept only as keyed HMACs.
type OtpChallenge struct{ ent.Schema }

func (OtpChallenge) Mixin() []ent.Mixin {
	return []ent.Mixin{idMixin{}, createdAtMixin{}}
}

func (OtpChallenge) Fields() []ent.Field {
	return []ent.Field{
		// One pending challenge per email.
		field.Bytes("email_hash").MinLen(32).MaxLen(32).Unique().Immutable(),
		field.Bytes("code_hash").MinLen(32).MaxLen(32).Immutable(),
		field.Time("expires_at").Immutable(),
		field.Time("resend_after").Immutable(),
		field.Int("attempts").Default(0).NonNegative(),
	}
}

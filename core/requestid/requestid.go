// Package requestid carries one safe correlation identifier across HTTP,
// orchestration, and durable run creation without carrying request content.
package requestid

import (
	"context"

	"github.com/google/uuid"
)

const Header = "X-Fort-Request-ID"

type contextKey struct{}

func New() string { return uuid.NewString() }

func Valid(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed.String() == value
}

func With(ctx context.Context, value string) context.Context {
	if !Valid(value) {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, value)
}

func From(ctx context.Context) string {
	value, _ := ctx.Value(contextKey{}).(string)
	if !Valid(value) {
		return ""
	}
	return value
}

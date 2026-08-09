// Package transporttrust carries authenticated transport provenance across an
// in-process HTTP handler call. It deliberately has no wire or header form.
package transporttrust

import "context"

type trustedKey struct{}

// WithTrusted marks ctx after an authenticated transport has decoded a
// request. Callers must not use this for ordinary network ingress.
func WithTrusted(ctx context.Context) context.Context {
	return context.WithValue(ctx, trustedKey{}, struct{}{})
}

// Trusted reports whether an authenticated in-process transport marked ctx.
func Trusted(ctx context.Context) bool {
	_, ok := ctx.Value(trustedKey{}).(struct{})
	return ok
}

// Package source provides the host-agnostic abstraction commands use to read
// schema folders from upstream repositories, plus a centralised error shape
// that callers map to friendly, non-stacktrace messages.
//
// Concrete hosts (today only GitHub, see internal/github) implement Client.
// Per ADR 0004 the abstraction stays host-agnostic from the start so adding a
// second host (Bitbucket, GitLab, …) is additive rather than a rewrite.
package source

import "context"

// Client reads the list of schema files under schemas/ at the default branch
// HEAD of a source reference.
type Client interface {
	ListSchemas(ctx context.Context, ref Ref) ([]string, error)
}

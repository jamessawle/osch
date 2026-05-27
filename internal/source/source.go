// Package source provides the provider-agnostic abstraction commands use to read
// schema folders from upstream repositories, plus a centralised error shape
// that callers map to friendly, non-stacktrace messages.
//
// Concrete providers (today only GitHub, see internal/github) implement Client.
// Per ADR 0004 the abstraction stays provider-agnostic from the start so adding a
// second provider (Bitbucket, GitLab, …) is additive rather than a rewrite.
package source

import "context"

// Client reads the list of schema files under schemas/ at the default branch
// HEAD of a source reference.
type Client interface {
	ListSchemas(ctx context.Context, ref Ref) ([]string, error)
}

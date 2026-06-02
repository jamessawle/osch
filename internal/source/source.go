// Package source provides the provider-agnostic abstraction commands use to read
// schema folders from upstream repositories, plus a centralised error shape
// that callers map to friendly, non-stacktrace messages.
//
// Concrete providers (today only GitHub, see internal/github) implement Client.
// Per ADR 0004 the abstraction stays provider-agnostic from the start so adding a
// second provider (Bitbucket, GitLab, …) is additive rather than a rewrite.
package source

import "context"

// Client reads schema folders from an upstream repository at the default branch
// HEAD commit. Implementations live under internal/<provider>.
type Client interface {
	// ListSchemas resolves the default branch HEAD commit of ref and returns the
	// names of schema directories under schemas/ at that commit, together with the
	// resolved SHA.
	ListSchemas(ctx context.Context, ref Ref) (sha string, names []string, err error)

	// FetchSchemaFiles fetches every file under schemas/<name>/ at the given commit
	// SHA. The returned map is keyed by forward-slash relative path within the
	// schema folder, so the manifest written from it is stable across platforms.
	FetchSchemaFiles(ctx context.Context, ref Ref, sha, name string) (map[string][]byte, error)

	// LatestSHA resolves the default branch HEAD commit SHA for ref without
	// listing the schemas/ folder. It exists for drift detection where the
	// extra contents call ListSchemas performs would be misleading (e.g.
	// a repo whose schemas/ folder has moved would surface as "no schemas"
	// rather than the up-to-date/behind question the caller is asking).
	LatestSHA(ctx context.Context, ref Ref) (string, error)
}

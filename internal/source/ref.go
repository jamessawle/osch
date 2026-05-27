package source

import (
	"fmt"
	"strings"
)

// ProviderGitHub is the provider identifier for GitHub, the only provider wired today.
const ProviderGitHub = "github"

// Ref is a provider-agnostic reference to an upstream repository that holds a
// schemas/ folder.
type Ref struct {
	Provider string
	Owner    string
	Name     string
}

// String renders the ref in canonical "owner/name" form. Per ADR 0004 the CLI
// grammar is flat today, so no provider prefix is shown.
func (r Ref) String() string {
	return r.Owner + "/" + r.Name
}

// ParseRef parses a CLI source argument. Today only the flat "user/repo" form
// is accepted and the provider is always GitHub; per ADR 0004 a "<provider>:" prefix
// grammar is introduced additively when a second provider lands. It returns a
// friendly error when the argument is not in the expected form.
func ParseRef(s string) (Ref, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return Ref{}, fmt.Errorf("invalid repository %q: expected user/repo form (e.g. openspec/schemas)", s)
	}
	owner, name := parts[0], parts[1]
	if owner == "" || name == "" || strings.ContainsAny(s, " \t\n") {
		return Ref{}, fmt.Errorf("invalid repository %q: expected user/repo form (e.g. openspec/schemas)", s)
	}
	return Ref{Provider: ProviderGitHub, Owner: owner, Name: name}, nil
}

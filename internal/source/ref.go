package source

import (
	"fmt"
	"strings"
)

// HostGitHub is the host identifier for GitHub, the only host wired today.
const HostGitHub = "github"

// Ref is a host-agnostic reference to an upstream repository that holds a
// schemas/ folder.
type Ref struct {
	Host  string
	Owner string
	Name  string
}

// String renders the ref in canonical "owner/name" form. Per ADR 0004 the CLI
// grammar is flat today, so no host prefix is shown.
func (r Ref) String() string {
	return r.Owner + "/" + r.Name
}

// ParseRef parses a CLI source argument. Today only the flat "user/repo" form
// is accepted and the host is always GitHub; per ADR 0004 a "<host>:" prefix
// grammar is introduced additively when a second host lands. It returns a
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
	return Ref{Host: HostGitHub, Owner: owner, Name: name}, nil
}

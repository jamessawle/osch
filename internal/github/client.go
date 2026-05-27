// Package github provides a small client for reading schema folders from
// GitHub repositories, plus a centralised error shape that callers map to
// friendly, non-stacktrace messages. Commands such as `add` and (later)
// `update` share this package so upstream failures are reported consistently.
package github

import (
	"context"
	"fmt"
	"strings"
)

// Repo identifies an upstream GitHub repository.
type Repo struct {
	Owner string
	Name  string
}

// String renders the repo in canonical "owner/name" form.
func (r Repo) String() string {
	return r.Owner + "/" + r.Name
}

// ParseRepo parses a "user/repo" argument. It returns a friendly error when
// the argument is not in that exact form.
func ParseRepo(s string) (Repo, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return Repo{}, fmt.Errorf("invalid repository %q: expected user/repo form (e.g. openspec/schemas)", s)
	}
	owner, name := parts[0], parts[1]
	if owner == "" || name == "" || strings.ContainsAny(s, " \t\n") {
		return Repo{}, fmt.Errorf("invalid repository %q: expected user/repo form (e.g. openspec/schemas)", s)
	}
	return Repo{Owner: owner, Name: name}, nil
}

// Client reads the list of schema files under schemas/ at the default branch
// HEAD of a repository.
type Client interface {
	ListSchemas(ctx context.Context, repo Repo) ([]string, error)
}

// ErrorKind classifies the upstream failures the client can surface.
type ErrorKind int

const (
	// KindNotFound means the repo does not exist or is not accessible (404).
	KindNotFound ErrorKind = iota + 1
	// KindNoSchemas means the repo exists but has no schemas/ folder.
	KindNoSchemas
	// KindEmptySchemas means the schemas/ folder exists but contains no files.
	KindEmptySchemas
	// KindNetwork means a transport-level failure occurred (timeout, DNS, etc).
	KindNetwork
)

// ClientError is the centralised error shape for upstream failures. Its
// message is friendly and free of stack traces; the underlying cause (when
// any) is available via errors.Unwrap.
type ClientError struct {
	Kind ErrorKind
	Repo Repo
	Err  error
}

func (e *ClientError) Error() string {
	switch e.Kind {
	case KindNotFound:
		return fmt.Sprintf("repository %q not found or not accessible", e.Repo)
	case KindNoSchemas:
		return fmt.Sprintf("repository %q has no schemas/ folder at the default branch", e.Repo)
	case KindEmptySchemas:
		return fmt.Sprintf("repository %q has an empty schemas/ folder", e.Repo)
	case KindNetwork:
		return fmt.Sprintf("could not reach GitHub for %q: %v", e.Repo, e.Err)
	default:
		return fmt.Sprintf("upstream error for %q: %v", e.Repo, e.Err)
	}
}

// Unwrap exposes the underlying cause so errors.Is/As keep working.
func (e *ClientError) Unwrap() error { return e.Err }

// NotFoundError reports that the repo does not exist or is not accessible.
func NotFoundError(repo Repo) *ClientError {
	return &ClientError{Kind: KindNotFound, Repo: repo}
}

// NoSchemasError reports that the repo has no schemas/ folder.
func NoSchemasError(repo Repo) *ClientError {
	return &ClientError{Kind: KindNoSchemas, Repo: repo}
}

// EmptySchemasError reports that the schemas/ folder exists but is empty.
func EmptySchemasError(repo Repo) *ClientError {
	return &ClientError{Kind: KindEmptySchemas, Repo: repo}
}

// NetworkError wraps a transport-level failure contacting GitHub.
func NetworkError(repo Repo, err error) *ClientError {
	return &ClientError{Kind: KindNetwork, Repo: repo, Err: err}
}

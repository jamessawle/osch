package source

import "fmt"

// ErrorKind classifies the upstream failures a Client can surface.
type ErrorKind int

const (
	// KindNotFound means the ref does not exist or is not accessible (404).
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
	Ref  Ref
	Err  error
}

func (e *ClientError) Error() string {
	switch e.Kind {
	case KindNotFound:
		return fmt.Sprintf("repository %q not found or not accessible", e.Ref)
	case KindNoSchemas:
		return fmt.Sprintf("repository %q has no schemas/ folder at the default branch", e.Ref)
	case KindEmptySchemas:
		return fmt.Sprintf("repository %q has an empty schemas/ folder", e.Ref)
	case KindNetwork:
		return fmt.Sprintf("could not reach %s for %q: %v", hostDisplayName(e.Ref.Host), e.Ref, e.Err)
	default:
		return fmt.Sprintf("upstream error for %q: %v", e.Ref, e.Err)
	}
}

// Unwrap exposes the underlying cause so errors.Is/As keep working.
func (e *ClientError) Unwrap() error { return e.Err }

// hostDisplayName maps a host identifier to its human-friendly name for use in
// messages. Unknown hosts are shown verbatim.
func hostDisplayName(host string) string {
	switch host {
	case HostGitHub:
		return "GitHub"
	default:
		return host
	}
}

// NotFoundError reports that the ref does not exist or is not accessible.
func NotFoundError(ref Ref) *ClientError {
	return &ClientError{Kind: KindNotFound, Ref: ref}
}

// NoSchemasError reports that the repo has no schemas/ folder.
func NoSchemasError(ref Ref) *ClientError {
	return &ClientError{Kind: KindNoSchemas, Ref: ref}
}

// EmptySchemasError reports that the schemas/ folder exists but is empty.
func EmptySchemasError(ref Ref) *ClientError {
	return &ClientError{Kind: KindEmptySchemas, Ref: ref}
}

// NetworkError wraps a transport-level failure contacting the host.
func NetworkError(ref Ref, err error) *ClientError {
	return &ClientError{Kind: KindNetwork, Ref: ref, Err: err}
}

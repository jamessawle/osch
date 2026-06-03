// Package chef defines the wire contract between the orchestrator
// (loop.sh) and any Chef impl. Each Chef impl lives in a subpackage.
package chef

// ProofStatus indicates the result of a chef execution.
type ProofStatus string

const (
	// StatusOK indicates the chef succeeded.
	StatusOK ProofStatus = "ok"
	// StatusFailed indicates the chef failed.
	StatusFailed ProofStatus = "failed"
)

// Chit is the orchestrator's request to a Chef to execute a task.
type Chit struct {
	Kind string   `json:"kind"`
	Task ChitTask `json:"task"`
	Repo ChitRepo `json:"repo"`
}

// ChitTask contains the issue and specification to implement.
type ChitTask struct {
	Ref           ChitRef `json:"ref"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	Specification string  `json:"specification"`
}

// ChitRef is the issue reference (source + id).
type ChitRef struct {
	Source string `json:"source"`
	ID     string `json:"id"`
}

// ChitRepo is the repository to work on.
type ChitRepo struct {
	Path string `json:"path"`
}

// Proof is the Chef's response, indicating success or failure.
type Proof struct {
	Kind       string      `json:"kind"`
	Status     ProofStatus `json:"status"`
	PR         *ProofPR    `json:"pr,omitempty"`
	Message    string      `json:"message,omitempty"`
	OutputTail string      `json:"output_tail,omitempty"`
}

// ProofPR contains pull request details on success.
type ProofPR struct {
	URL    string `json:"url"`
	Number int    `json:"number"`
}

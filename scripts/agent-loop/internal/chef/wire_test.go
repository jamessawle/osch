package chef_test

import (
	"encoding/json"
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChit_UnmarshalRoundTrip(t *testing.T) {
	t.Parallel()
	in := []byte(`{
		"kind": "implement",
		"task": {
			"ref": {"source": "github", "id": "123"},
			"title": "Fix things",
			"description": "Body of the issue.",
			"specification": "## Agent Brief\n..."
		},
		"repo": {"path": "/tmp/source"}
	}`)

	var c chef.Chit
	require.NoError(t, json.Unmarshal(in, &c))
	assert.Equal(t, "implement", c.Kind)
	assert.Equal(t, "github", c.Task.Ref.Source)
	assert.Equal(t, "123", c.Task.Ref.ID)
	assert.Equal(t, "Fix things", c.Task.Title)
	assert.Equal(t, "Body of the issue.", c.Task.Description)
	assert.Equal(t, "## Agent Brief\n...", c.Task.Specification)
	assert.Equal(t, "/tmp/source", c.Repo.Path)

	out, err := json.Marshal(c)
	require.NoError(t, err)
	var c2 chef.Chit
	require.NoError(t, json.Unmarshal(out, &c2))
	assert.Equal(t, c, c2)
}

func TestProof_SuccessMarshalsFlatPR(t *testing.T) {
	t.Parallel()
	p := chef.Proof{
		Kind:   "implement",
		Status: chef.StatusOK,
		PR:     &chef.ProofPR{URL: "https://example.com/pr/1", Number: 1},
	}
	out, err := json.Marshal(p)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"kind":"implement",
		"status":"ok",
		"pr":{"url":"https://example.com/pr/1","number":1}
	}`, string(out))
}

func TestProof_FailureOmitsPR(t *testing.T) {
	t.Parallel()
	p := chef.Proof{
		Kind:       "implement",
		Status:     chef.StatusFailed,
		Message:    "setup: command exited 1",
		OutputTail: "error: build failed",
	}
	out, err := json.Marshal(p)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"kind":"implement",
		"status":"failed",
		"message":"setup: command exited 1",
		"output_tail":"error: build failed"
	}`, string(out))
}

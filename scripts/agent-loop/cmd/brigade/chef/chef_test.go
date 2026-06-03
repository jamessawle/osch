package chef_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	chefcmd "github.com/jamessawle/osch/scripts/agent-loop/cmd/brigade/chef"
	wirechef "github.com/jamessawle/osch/scripts/agent-loop/internal/chef"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChef_UnknownChefNameExitsNonZero(t *testing.T) {
	t.Parallel()
	var stdout, stderr bytes.Buffer
	cmd := chefcmd.Cmd{Name: "designer"}
	err := cmd.Run(context.Background(), chefcmd.IO{
		Stdin:  bytes.NewReader([]byte(`{"kind":"implement"}`)),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.Error(t, err)
	assert.Empty(t, stdout.String())
}

func TestChef_EngineerNameReadsChitAndWritesProof(t *testing.T) {
	t.Parallel()
	chit := wirechef.Chit{
		Kind: "design",
		Task: wirechef.ChitTask{Ref: wirechef.ChitRef{Source: "github", ID: "1"}},
		Repo: wirechef.ChitRepo{Path: t.TempDir()},
	}
	raw, _ := json.Marshal(chit)

	var stdout, stderr bytes.Buffer
	cmd := chefcmd.Cmd{Name: "engineer"}
	err := cmd.Run(context.Background(), chefcmd.IO{
		Stdin:  bytes.NewReader(raw),
		Stdout: &stdout,
		Stderr: &stderr,
	})
	require.NoError(t, err)

	var p wirechef.Proof
	require.NoError(t, json.Unmarshal(stdout.Bytes(), &p))
	assert.Equal(t, wirechef.StatusFailed, p.Status)
	assert.Contains(t, p.Message, "unsupported kind")
}

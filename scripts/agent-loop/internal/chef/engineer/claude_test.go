package engineer_test

import (
	"encoding/json"
	"testing"

	"github.com/jamessawle/osch/scripts/agent-loop/internal/chef/engineer"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedSettings_ValidJSON(t *testing.T) {
	t.Parallel()
	raw := engineer.SettingsJSON()
	require.NotEmpty(t, raw)
	var v any
	require.NoError(t, json.Unmarshal(raw, &v))
}

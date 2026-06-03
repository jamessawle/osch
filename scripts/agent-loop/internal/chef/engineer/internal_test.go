package engineer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStripCodeFence(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"no fence", "plain text\nmore text", "plain text\nmore text"},
		{"plain triple-fence wrap", "```\ninner\n```", "inner"},
		{"language-tagged fence", "```md\n# title\nbody\n```", "# title\nbody"},
		{"trailing blank lines after close", "```\ninner\n```\n\n", "inner"},
		{"missing closing fence returns original", "```md\nopen but never closed", "```md\nopen but never closed"},
		{"opening fence with no newline", "```", "```"},
		{"inner fences left alone", "```\nfoo\n```bar```\n```", "foo\n```bar```"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, stripCodeFence(tc.in))
		})
	}
}

func TestBuildImplementPrompt(t *testing.T) {
	t.Parallel()
	t.Run("includes brief when non-empty and states precedence", func(t *testing.T) {
		t.Parallel()
		in := ImplementInput{
			TaskRef:       TaskRef{Source: "github", ID: "42"},
			Title:         "Fix the thing",
			Description:   "Original issue body.",
			Specification: "Detailed brief here.",
		}
		got := buildImplementPrompt(in, []string{"go test ./..."}, "")
		assert.Contains(t, got, "issue #42")
		assert.Contains(t, got, "Fix the thing")
		assert.Contains(t, got, "Original issue body.")
		assert.Contains(t, got, "Detailed brief here.")
		assert.Contains(t, got, "brief is the exclusive source of truth")
		assert.Contains(t, got, "DO NOT push")
		assert.Contains(t, got, "Refs #42")
		assert.Contains(t, got, "  $ go test ./...")
	})

	t.Run("omits brief block when specification is empty", func(t *testing.T) {
		t.Parallel()
		in := ImplementInput{
			TaskRef:     TaskRef{Source: "github", ID: "7"},
			Title:       "x",
			Description: "y",
		}
		got := buildImplementPrompt(in, nil, "")
		assert.NotContains(t, got, "Agent brief")
		assert.NotContains(t, got, "exclusive source of truth")
	})

	t.Run("feeds failing-checks output back into prompt", func(t *testing.T) {
		t.Parallel()
		in := ImplementInput{TaskRef: TaskRef{Source: "github", ID: "1"}, Title: "t"}
		got := buildImplementPrompt(in, []string{"go test"}, "--- FAILED: go test ---\nFAIL TestX\n")
		assert.Contains(t, got, "Prior attempt left the following checks failing")
		assert.Contains(t, got, "FAIL TestX")
	})
}

func TestAppendCloses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		id   string
		want string
	}{
		{"plain body gets closes appended", "Did the thing.", "42", "Did the thing.\n\nCloses #42"},
		{"already has Closes for same id leaves body alone", "Body.\n\nCloses #42", "42", "Body.\n\nCloses #42"},
		{"lowercase fixes keyword matches", "Body. fixes #42 today.", "42", "Body. fixes #42 today."},
		{"resolved keyword matches", "Resolved #42 yesterday.", "42", "Resolved #42 yesterday."},
		{"Closes for different id still appends", "Closes #99", "42", "Closes #99\n\nCloses #42"},
		{"no keyword even if #42 mentioned still appends", "See #42 for context.", "42", "See #42 for context.\n\nCloses #42"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, appendCloses(tc.body, tc.id))
		})
	}
}

func TestTailBufferRetainsTail(t *testing.T) {
	t.Parallel()
	tb := &tailBuffer{max: 8}
	_, _ = tb.Write([]byte("abcdef"))
	assert.Equal(t, "abcdef", tb.String())
	_, _ = tb.Write([]byte("ghij"))
	assert.Equal(t, "cdefghij", tb.String(), "should retain last 8 bytes")
	// large write in one go
	tb2 := &tailBuffer{max: 4}
	_, _ = tb2.Write([]byte(strings.Repeat("x", 100)))
	assert.Equal(t, "xxxx", tb2.String())
}

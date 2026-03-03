package enrichment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStripCodeFence_NoFence(t *testing.T) {
	input := "# Hello\nWorld"
	assert.Equal(t, "# Hello\nWorld", stripCodeFence(input))
}

func TestStripCodeFence_MarkdownFence(t *testing.T) {
	input := "```markdown\n# Hello\nWorld\n```"
	assert.Equal(t, "# Hello\nWorld", stripCodeFence(input))
}

func TestStripCodeFence_PlainFence(t *testing.T) {
	input := "```\n# Hello\nWorld\n```"
	assert.Equal(t, "# Hello\nWorld", stripCodeFence(input))
}

func TestStripCodeFence_WithSurroundingWhitespace(t *testing.T) {
	input := "  \n```markdown\n# Hello\n```\n  "
	assert.Equal(t, "# Hello", stripCodeFence(input))
}

func TestStripCodeFence_OnlyOpeningFenceNoNewline(t *testing.T) {
	input := "```no newline"
	assert.Equal(t, input, stripCodeFence(input))
}

func TestExtractJSON_PureJSON(t *testing.T) {
	input := `{"pages":[{"slug":"overview"}]}`
	assert.Equal(t, input, extractJSON(input))
}

func TestExtractJSON_SurroundingText(t *testing.T) {
	input := "Here is the plan:\n{\"pages\":[]}\nDone."
	assert.Equal(t, `{"pages":[]}`, extractJSON(input))
}

func TestExtractJSON_MarkdownFence(t *testing.T) {
	input := "```json\n{\"pages\":[]}\n```"
	assert.Equal(t, `{"pages":[]}`, extractJSON(input))
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	input := `{"pages":[{"slug":"a","children":[{"slug":"b"}]}]}`
	assert.Equal(t, input, extractJSON(input))
}

func TestExtractJSON_BracesInStrings(t *testing.T) {
	input := `{"title":"func() { return }"}`
	assert.Equal(t, input, extractJSON(input))
}

func TestExtractJSON_NoJSON(t *testing.T) {
	input := "no json here"
	assert.Equal(t, input, extractJSON(input))
}

func TestExtractJSON_EscapedQuotesInStrings(t *testing.T) {
	input := `{"title":"say \"hello\""}`
	assert.Equal(t, input, extractJSON(input))
}

func TestRepairJSON_Valid(t *testing.T) {
	input := `{"pages":[{"slug":"a"},{"slug":"b"}]}`
	assert.Equal(t, input, repairJSON(input))
}

func TestRepairJSON_MissingCommaObjects(t *testing.T) {
	input := `{"pages":[{"slug":"a"}{"slug":"b"}]}`
	expected := `{"pages":[{"slug":"a"},{"slug":"b"}]}`
	assert.Equal(t, expected, repairJSON(input))
}

func TestRepairJSON_MissingCommaWithWhitespace(t *testing.T) {
	input := "{\"pages\":[{\"slug\":\"a\"} \n {\"slug\":\"b\"}]}"
	result := repairJSON(input)
	assert.True(t, json.Valid([]byte(result)), "should produce valid JSON, got: %s", result)
}

func TestRepairJSON_MissingCommaArrays(t *testing.T) {
	input := `{"a":["x"]["y"]}`
	expected := `{"a":["x"],["y"]}`
	assert.Equal(t, expected, repairJSON(input))
}

func TestRepairJSON_BracesInStrings(t *testing.T) {
	input := `{"title":"}{not a break"}`
	assert.Equal(t, input, repairJSON(input))
}

func TestRepairJSON_NestedChildren(t *testing.T) {
	// Simulates the exact error from the bug report.
	input := `{"pages":[{"slug":"overview","children":[{"slug":"features"}{"slug":"getting-started"}]}]}`
	result := repairJSON(input)
	require.True(t, json.Valid([]byte(result)), "should produce valid JSON, got: %s", result)

	var outline wikiOutline
	require.NoError(t, json.Unmarshal([]byte(result), &outline))
	assert.Len(t, outline.Pages[0].Children, 2)
}

func TestWikiOutline_Flatten(t *testing.T) {
	outline := wikiOutline{
		Pages: []outlinePage{
			{
				Slug:  "overview",
				Title: "Overview",
				Children: []outlinePage{
					{Slug: "install", Title: "Install"},
				},
			},
			{
				Slug:  "api",
				Title: "API",
			},
		},
	}

	flat := outline.flatten()
	require.Len(t, flat, 3)
	assert.Equal(t, "overview", flat[0].Slug)
	assert.Equal(t, "install", flat[1].Slug)
	assert.Equal(t, "api", flat[2].Slug)
}

func TestWikiOutline_FlattenEmpty(t *testing.T) {
	outline := wikiOutline{}
	assert.Empty(t, outline.flatten())
}

func TestWikiOutline_FlattenDeeplyNested(t *testing.T) {
	outline := wikiOutline{
		Pages: []outlinePage{
			{
				Slug: "a",
				Children: []outlinePage{
					{
						Slug: "b",
						Children: []outlinePage{
							{Slug: "c"},
						},
					},
				},
			},
		},
	}

	flat := outline.flatten()
	require.Len(t, flat, 3)
	assert.Equal(t, "a", flat[0].Slug)
	assert.Equal(t, "b", flat[1].Slug)
	assert.Equal(t, "c", flat[2].Slug)
}

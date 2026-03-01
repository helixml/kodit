package wiki

import (
	"regexp"
	"strings"
)

var markdownLink = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

// RewrittenContent is a value object that holds markdown content with
// internal wiki links rewritten to use full hierarchical API paths.
type RewrittenContent struct {
	text string
}

// NewRewrittenContent rewrites markdown links in content. Links whose target
// is a known slug are replaced with urlPrefix + "/" + fullPath + suffix.
// Absolute URLs and unknown slugs are left unchanged.
func NewRewrittenContent(content string, pathIndex map[string]string, urlPrefix string, suffix string) RewrittenContent {
	result := markdownLink.ReplaceAllStringFunc(content, func(match string) string {
		parts := markdownLink.FindStringSubmatch(match)
		if len(parts) < 3 {
			return match
		}
		label := parts[1]
		target := parts[2]

		if strings.HasPrefix(target, "http://") ||
			strings.HasPrefix(target, "https://") ||
			strings.HasPrefix(target, "/") {
			return match
		}

		if fullPath, ok := pathIndex[target]; ok {
			return "[" + label + "](" + urlPrefix + "/" + fullPath + suffix + ")"
		}

		return match
	})
	return RewrittenContent{text: result}
}

// String returns the rewritten markdown content.
func (r RewrittenContent) String() string { return r.text }

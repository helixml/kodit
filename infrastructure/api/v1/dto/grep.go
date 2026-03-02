package dto

// GrepMatchSchema represents a single line match from grep.
type GrepMatchSchema struct {
	Line    int    `json:"line"`
	Content string `json:"content"`
}

// GrepFileSchema represents grep results for a single file.
type GrepFileSchema struct {
	Path     string            `json:"path"`
	Language string            `json:"language"`
	Matches  []GrepMatchSchema `json:"matches"`
}

// GrepResponse is the response body for the grep endpoint.
type GrepResponse struct {
	Data []GrepFileSchema `json:"data"`
}

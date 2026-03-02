package git

import (
	"testing"
)

func TestParseLsTree(t *testing.T) {
	t.Run("parses files with sizes", func(t *testing.T) {
		output := "100644 blob abc123def456 1234\tREADME.md\n" +
			"100644 blob def789abc012  567\tsrc/main.go\n"

		files := parseLsTree(output)

		if len(files) != 2 {
			t.Fatalf("expected 2 files, got %d", len(files))
		}

		if files[0].Path != "README.md" {
			t.Errorf("expected README.md, got %s", files[0].Path)
		}
		if files[0].BlobSHA != "abc123def456" {
			t.Errorf("expected abc123def456, got %s", files[0].BlobSHA)
		}
		if files[0].Size != 1234 {
			t.Errorf("expected size 1234, got %d", files[0].Size)
		}

		if files[1].Path != "src/main.go" {
			t.Errorf("expected src/main.go, got %s", files[1].Path)
		}
		if files[1].Size != 567 {
			t.Errorf("expected size 567, got %d", files[1].Size)
		}
	})

	t.Run("empty output returns nil", func(t *testing.T) {
		files := parseLsTree("")
		if files != nil {
			t.Fatalf("expected nil, got %v", files)
		}
	})

	t.Run("skips non-blob entries", func(t *testing.T) {
		output := "040000 tree abc123def456       -\tsrc\n" +
			"100644 blob def789abc012    100\tsrc/main.go\n"

		files := parseLsTree(output)
		if len(files) != 1 {
			t.Fatalf("expected 1 file, got %d", len(files))
		}
		if files[0].Path != "src/main.go" {
			t.Errorf("expected src/main.go, got %s", files[0].Path)
		}
	})

	t.Run("handles whitespace-only output", func(t *testing.T) {
		files := parseLsTree("   \n  \n")
		if files != nil {
			t.Fatalf("expected nil, got %v", files)
		}
	})
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "src/main.go", false},
		{"**/*.go", "main.go", true},
		{"**/*.go", "src/main.go", true},
		{"**/*.go", "src/pkg/main.go", true},
		{"**/*.go", "README.md", false},
		{"*", "README.md", true},
		{"*", "src/main.go", false},
		{"src/*.go", "src/main.go", true},
		{"src/*.go", "src/pkg/main.go", false},
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/pkg/main.go", true},
		{"src/**/*.go", "README.md", false},
		{"**", "any/path/here.txt", true},
		{"**", "file.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

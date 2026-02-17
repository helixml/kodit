package kodit

import (
	"testing"
)

func TestVectorchordDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "appends search_path when absent",
			dsn:  "postgresql://postgres:secret@localhost:5432/kodit",
			want: "postgresql://postgres:secret@localhost:5432/kodit?search_path=public%2Cbm25_catalog%2Ctokenizer_catalog",
		},
		{
			name: "preserves existing query parameters",
			dsn:  "postgresql://postgres:secret@localhost:5432/kodit?sslmode=disable",
			want: "postgresql://postgres:secret@localhost:5432/kodit?search_path=public%2Cbm25_catalog%2Ctokenizer_catalog&sslmode=disable",
		},
		{
			name: "does not override user-provided search_path",
			dsn:  "postgresql://postgres:secret@localhost:5432/kodit?search_path=custom",
			want: "postgresql://postgres:secret@localhost:5432/kodit?search_path=custom",
		},
		{
			name: "detects search_path inside options parameter",
			dsn:  "postgresql://postgres:secret@localhost:5432/kodit?options=-c%20search_path%3Dpublic",
			want: "postgresql://postgres:secret@localhost:5432/kodit?options=-c%20search_path%3Dpublic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := vectorchordDSN(tt.dsn)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("vectorchordDSN(%q)\n got  %q\n want %q", tt.dsn, got, tt.want)
			}
		})
	}
}

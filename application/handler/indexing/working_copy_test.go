package indexing

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/helixml/kodit/domain/repository"
	"github.com/helixml/kodit/infrastructure/persistence"
)

// saveFileOnDisk saves the file record and writes a placeholder regular file
// at its path inside the working copy dir, because handlers only hand paths
// of regular files in the working copy to document renderers.
func saveFileOnDisk(t *testing.T, ctx context.Context, store persistence.FileStore, dir string, f repository.File) (repository.File, error) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(f.Path()))
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("placeholder"), 0o644))
	return store.Save(ctx, f)
}

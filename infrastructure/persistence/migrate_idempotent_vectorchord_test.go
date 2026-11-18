//go:build integration

package persistence

import (
	"context"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/helixml/kodit/internal/database"
)

type sqlRecorder struct {
	logger.Interface
	mu   sync.Mutex
	stmt []string
}

func (r *sqlRecorder) LogMode(logger.LogLevel) logger.Interface { return r }

func (r *sqlRecorder) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	r.mu.Lock()
	r.stmt = append(r.stmt, sql)
	r.mu.Unlock()
}

// TestMigrate_SecondRunIssuesNoDDL guards against migrations that change a
// migrated database on every start. ALTER TABLE takes an ACCESS EXCLUSIVE
// lock even when it changes nothing, so a start next to a running instance,
// or behind a pg_dump, would stall on it.
//
//	VECTORCHORD_TEST_URL="postgresql://postgres:mysecretpassword@localhost:5432/kodit" \
//	  go test -tags integration -v -run TestMigrate_SecondRunIssuesNoDDL ./infrastructure/persistence/
func TestMigrate_SecondRunIssuesNoDDL(t *testing.T) {
	dsn := os.Getenv("VECTORCHORD_TEST_URL")
	if dsn == "" {
		t.Skip("VECTORCHORD_TEST_URL not set")
	}
	ctx := context.Background()

	db, err := database.NewDatabase(ctx, dsn)
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	require.NoError(t, PreMigrate(db))
	require.NoError(t, AutoMigrate(db))

	rec := &sqlRecorder{Interface: logger.Discard}
	quiet, err := database.NewDatabaseWithConfig(ctx, dsn, &gorm.Config{Logger: rec})
	require.NoError(t, err)
	defer func() { _ = quiet.Close() }()
	require.NoError(t, PreMigrate(quiet))
	require.NoError(t, AutoMigrate(quiet))

	write := regexp.MustCompile(`(?i)^\s*(ALTER|CREATE|DROP|UPDATE|INSERT|DELETE)\b`)
	var writes []string
	for _, s := range rec.stmt {
		if write.MatchString(s) {
			writes = append(writes, strings.Join(strings.Fields(s), " "))
		}
	}
	require.Empty(t, writes, "second migration run changed the schema")
}

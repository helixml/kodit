package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// slogGormLogger adapts slog to GORM's logger.Interface so that every SQL
// query executed by GORM is emitted as an slog.Debug message. Level filtering
// is delegated to slog — when the configured slog level is above Debug the
// messages are silently discarded and the SQL formatting callback is never
// invoked (avoiding overhead in production).
type slogGormLogger struct{}

// LogMode is a no-op; level filtering is handled by slog.
func (l slogGormLogger) LogMode(logger.LogLevel) logger.Interface { return l }

// Info logs informational messages from GORM.
func (l slogGormLogger) Info(_ context.Context, msg string, args ...any) {
	slog.Info(fmt.Sprintf(msg, args...))
}

// Warn logs warning messages from GORM.
func (l slogGormLogger) Warn(_ context.Context, msg string, args ...any) {
	slog.Warn(fmt.Sprintf(msg, args...))
}

// Error logs error messages from GORM.
func (l slogGormLogger) Error(_ context.Context, msg string, args ...any) {
	slog.Error(fmt.Sprintf(msg, args...))
}

// maxSQLLength is the maximum length of a SQL string in debug logs before
// it gets truncated with an ellipsis.
const maxSQLLength = 200

// truncateSQL shortens a SQL string for readable log output, replacing the
// middle with "..." when it exceeds maxSQLLength.
func truncateSQL(sql string) string {
	if len(sql) <= maxSQLLength {
		return sql
	}
	half := (maxSQLLength - 3) / 2
	return sql[:half] + "..." + sql[len(sql)-half:]
}

// Trace is called by GORM after every SQL operation. Real errors are logged at
// Error level. ErrRecordNotFound is not an error — it is the normal "no rows"
// result from .First() — and is logged at Debug level alongside successful
// queries. Debug messages are only emitted when the slog level allows it,
// avoiding the cost of formatting the SQL string in production.
func (l slogGormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		sql, rows := fc()
		slog.Error("gorm query error",
			"sql", truncateSQL(sql),
			"rows", rows,
			"duration", elapsed,
			"error", err,
		)
		return
	}

	if !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}

	sql, rows := fc()
	slog.Debug("gorm query",
		"sql", truncateSQL(sql),
		"rows", rows,
		"duration", elapsed,
	)
}

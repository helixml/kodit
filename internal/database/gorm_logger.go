package database

import (
	"context"
	"errors"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// zerologGormLogger adapts zerolog to GORM's logger.Interface so that every SQL
// query executed by GORM is emitted as a debug message. Level filtering
// is delegated to zerolog — when the configured level is above Debug the
// messages are silently discarded and the SQL formatting callback is never
// invoked (avoiding overhead in production).
type zerologGormLogger struct{}

// LogMode is a no-op; level filtering is handled by zerolog.
func (l zerologGormLogger) LogMode(logger.LogLevel) logger.Interface { return l }

// Info logs informational messages from GORM.
func (l zerologGormLogger) Info(_ context.Context, msg string, args ...any) {
	log.Info().Msgf(msg, args...)
}

// Warn logs warning messages from GORM.
func (l zerologGormLogger) Warn(_ context.Context, msg string, args ...any) {
	log.Warn().Msgf(msg, args...)
}

// Error logs error messages from GORM.
func (l zerologGormLogger) Error(_ context.Context, msg string, args ...any) {
	log.Error().Msgf(msg, args...)
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
// queries. Debug messages are only emitted when the zerolog level allows it,
// avoiding the cost of formatting the SQL string in production.
func (l zerologGormLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	elapsed := time.Since(begin)

	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		sql, rows := fc()
		log.Error().
			Str("sql", truncateSQL(sql)).
			Int64("rows", rows).
			Dur("duration", elapsed).
			Err(err).
			Msg("gorm query error")
		return
	}

	if zerolog.GlobalLevel() > zerolog.DebugLevel {
		return
	}

	sql, rows := fc()
	log.Debug().
		Str("sql", truncateSQL(sql)).
		Int64("rows", rows).
		Dur("duration", elapsed).
		Msg("gorm query")
}

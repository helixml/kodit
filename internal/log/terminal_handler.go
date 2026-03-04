package log

import (
	"fmt"
	"io"
	"strings"

	"github.com/rs/zerolog"
)

const (
	ansiReset  = "\033[0m"
	ansiDim    = "\033[2m"
	ansiBold   = "\033[1m"
	ansiRed    = "\033[31m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
	ansiCyan   = "\033[36m"
)

// newConsoleWriter creates a zerolog.ConsoleWriter with coloured terminal output.
//
// Output format:
//
//	15:04:05.000 INF server started port=8080
func newConsoleWriter(w io.Writer) zerolog.ConsoleWriter {
	return zerolog.ConsoleWriter{
		Out:        w,
		TimeFormat: "15:04:05.000",
		FormatLevel: func(i interface{}) string {
			level, _ := i.(string)
			switch strings.ToUpper(level) {
			case "DEBUG":
				return ansiCyan + "DBG" + ansiReset
			case "INFO":
				return ansiGreen + "INF" + ansiReset
			case "WARN":
				return ansiYellow + "WRN" + ansiReset
			case "ERROR":
				return ansiRed + "ERR" + ansiReset
			default:
				return level
			}
		},
		FormatMessage: func(i interface{}) string {
			return fmt.Sprintf("%s%v%s", ansiBold, i, ansiReset)
		},
		FormatFieldName: func(i interface{}) string {
			return fmt.Sprintf("%s%v=%s", ansiDim, i, ansiReset)
		},
		FormatTimestamp: func(i interface{}) string {
			return fmt.Sprintf("%s%v%s", ansiDim, i, ansiReset)
		},
	}
}

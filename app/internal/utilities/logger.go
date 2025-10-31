package utilities

import (
	"io"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitLogger はログ設定を初期化する
func InitLogger(level string) {
	// ログレベルの設定
	var logLevel zerolog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		logLevel = zerolog.DebugLevel
	case "INFO":
		logLevel = zerolog.InfoLevel
	case "WARN", "WARNING":
		logLevel = zerolog.WarnLevel
	case "ERROR":
		logLevel = zerolog.ErrorLevel
	case "FATAL":
		logLevel = zerolog.FatalLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	// 読みやすい形式（開発用）
	// 本番では JSON 形式が推奨
	output := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}

	log.Logger = zerolog.New(output).With().Timestamp().Caller().Logger()
}

// InitLoggerJSON はJSON形式でログを初期化（本番推奨）
func InitLoggerJSON(level string, output io.Writer) {
	var logLevel zerolog.Level
	switch strings.ToUpper(level) {
	case "DEBUG":
		logLevel = zerolog.DebugLevel
	case "INFO":
		logLevel = zerolog.InfoLevel
	case "WARN", "WARNING":
		logLevel = zerolog.WarnLevel
	case "ERROR":
		logLevel = zerolog.ErrorLevel
	case "FATAL":
		logLevel = zerolog.FatalLevel
	default:
		logLevel = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(logLevel)

	if output == nil {
		output = os.Stdout
	}

	log.Logger = zerolog.New(output).With().Timestamp().Caller().Logger()
}

// GetLogger はグローバルロガーを返す
func GetLogger() *zerolog.Logger {
	return &log.Logger
}

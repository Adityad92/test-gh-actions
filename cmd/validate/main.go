package main

import (
	"os"

	"github.com/Adityad92/test-gh-actions/pkg/utility"
	_ "github.com/joho/godotenv/autoload"
	log "github.com/sirupsen/logrus"
)

func main() {
	logger := log.New()

	logFormat := os.Getenv("LOG_FORMAT")
	switch logFormat {
	case "TEXT", "text", "Text":
		logger.SetFormatter(&log.TextFormatter{})
	case "JSON", "json", "Json":
		logger.SetFormatter(&log.JSONFormatter{})
	default:
		logger.SetFormatter(&log.TextFormatter{})
	}

	logLevel := os.Getenv("LOG_LEVEL")
	switch logLevel {
	case "trace", "Trace", "TRACE":
		logger.SetLevel(log.TraceLevel)
	case "debug", "Debug", "DEBUG":
		logger.SetLevel(log.DebugLevel)
	case "warn", "Warn", "WARN":
		logger.SetLevel(log.WarnLevel)
	case "error", "Error", "ERROR":
		logger.SetLevel(log.ErrorLevel)
	case "fatal", "Fatal", "FATAL":
		logger.SetLevel(log.FatalLevel)
	case "panic", "Panic", "PANIC":
		logger.SetLevel(log.PanicLevel)
	case "info", "Info", "INFO":
		fallthrough
	default:
		logger.SetLevel(log.InfoLevel)
	}

	report, isFailure, err := utility.Validate()
	if err != nil {
		logger.Fatalf("Validation error: %+v\n", err)
	}

	if isFailure {
		logger.Info("===============================:\n")
		logger.Error("Validation failed:\n" + report)
		os.Exit(1)
	} else {
		logger.Info("===============================:\n")
		logger.Info("Validation succeeded:\n" + report)
	}

}

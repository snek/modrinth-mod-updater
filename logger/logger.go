package logger

import (
	"log"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	Log       *zap.SugaredLogger
	ZapLogger *zap.Logger // Expose the raw zap Logger
)

func InitLogger() {
	// Configure the encoder
	encoderCfg := zapcore.EncoderConfig{
		TimeKey:        "T", // Keep time key brief
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "",              // Disable caller key
		FunctionKey:    zapcore.OmitKey, // Disable function key
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,                        // INFO, WARN, etc.
		EncodeTime:     zapcore.TimeEncoderOfLayout("2006-01-02 15:04:05"), // Simpler time format
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder, // Won't be used due to empty CallerKey
		// Customize how structured fields are encoded (key=value format)
		ConsoleSeparator: "  ", // Separator between elements in console output
	}

	// Configure the core for file logging
	logFile, err := os.OpenFile("modrinth-updater.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("can't open log file: %v", err)
	}
	fileWriter := zapcore.AddSync(logFile)

	// Create a core that writes INFO level and above logs to the file
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg), // Use ConsoleEncoder with custom config
		fileWriter,
		zap.InfoLevel, // Log InfoLevel and above to file
	)

	// Build the logger
	ZapLogger = zap.New(core) // Removed AddCaller() and AddStacktrace(zap.ErrorLevel) for cleaner output

	Log = ZapLogger.Sugar()
	Log.Info("Logger initialized, logging to modrinth-updater.log") // Log initialization message

	// Ensure the deferred Sync happens
	// The Sync function remains the same, but it should be called on shutdown (e.g., in main.go)
}

func Sync() {
	if ZapLogger != nil {
		_ = ZapLogger.Sync() // flushes buffer, if any
	}
}

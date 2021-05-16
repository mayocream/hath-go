package server

import (
	"fmt"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func initLogger(config Config) {
	conf := zap.NewProductionConfig()
	conf.EncoderConfig = zap.NewDevelopmentEncoderConfig()
	conf.DisableCaller = true
	if config.Debug {
		conf.DisableCaller = false
		conf.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	conf.Encoding = "console"
	conf.Level = zap.NewAtomicLevelAt(zapcore.WarnLevel)
	conf.Level.UnmarshalText([]byte(config.LogLevel))

	fmt.Println("logger level at: ", conf.Level.String())

	logger, err := conf.Build()
	if err != nil {
		panic(err)
	}

	zap.ReplaceGlobals(logger)
}
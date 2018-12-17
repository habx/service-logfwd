package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

var log *zap.SugaredLogger

func init() {
	initLog(false)
}

func initLog(dev bool) {
	var config zap.Config
	if dev {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	logger, _ := config.Build()
	log = logger.Sugar()
}

func main() {
	server := NewServer()

	log.Infow("Starting")

	if err := server.config.Load(); err != nil {
		log.Fatalw(
			"Couldn't load config",
			"err", err,
		)
		return
	}

	log.Infow(
		"Loaded config",
		"config", server.config,
	)

	if server.config.LogEnv == "dev" {
		initLog(true)
	}

	if _, err := server.listen(); err != nil {
		log.Fatalw("Can't listen", "err", err)
	}

	exit := <-server.exit

	os.Exit(exit)
}

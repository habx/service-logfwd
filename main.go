package main

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func getLog(dev bool) *zap.SugaredLogger {
	var config zap.Config
	if dev {
		config = zap.NewDevelopmentConfig()
	} else {
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zapcore.InfoLevel)
	}

	logger, _ := config.Build()
	return logger.Sugar()
}

func main() {
	log := getLog(false)

	log.Infow("Starting")

	config := NewConfig()

	if err := config.Load(); err != nil {
		log.Fatalw(
			"Couldn't load config",
			"err", err,
		)
		return
	}

	if config.LogEnv == "dev" {
		log = getLog(true)
	}

	server := NewServer(config, log)

	log.Infow(
		"Loaded config",
		"config", server.config,
	)

	if _, err := server.listen(); err != nil {
		log.Fatalw("Can't listen", "err", err)
	}

	exit := <-server.exit

	os.Exit(exit)
}

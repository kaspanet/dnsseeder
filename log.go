package main

import (
	"fmt"
	"os"

	"github.com/kaspanet/kaspad/infrastructure/logger"
	"github.com/kaspanet/kaspad/util/panics"
)

var (
	backendLog = logger.NewBackend()
	log        = backendLog.Logger("SEED")
	spawn      = panics.GoroutineWrapperFunc(log)
)

func initLog(noLogFiles bool, logLevel, logFile, errLogFile string) {
	level, ok := logger.LevelFromString(logLevel)
	if !ok {
		fmt.Fprintf(os.Stderr, "Invalid loglevel: %s", logLevel)
		os.Exit(1)
	}
	err := backendLog.AddLogWriter(os.Stdout, level)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error adding stdout to the logger for level %s: %s", logger.LevelWarn, err)
		os.Exit(1)
	}

	if !noLogFiles {
		err = backendLog.AddLogFile(logFile, logger.LevelTrace)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error adding log file %s as log rotator for level %s: %s", logFile, logger.LevelTrace, err)
			os.Exit(1)
		}
		err = backendLog.AddLogFile(errLogFile, logger.LevelWarn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error adding log file %s as log rotator for level %s: %s", errLogFile, logger.LevelWarn, err)
			os.Exit(1)
		}
	}

	err = backendLog.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error starting the logger: %s ", err)
		os.Exit(1)
	}

	log.SetLevel(logger.LevelDebug)
}

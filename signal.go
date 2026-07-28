package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func setupSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sig:
		case <-shutdownCh:
		}

		fmt.Println()
		logInfo(L("shutting_down"))

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if httpServer != nil {
			httpServer.Shutdown(ctx)
		}

		time.Sleep(100 * time.Millisecond)

		logMu.Lock()
		if logFile != nil {
			logFile.Close()
		}
		logMu.Unlock()

		if dataLogFile != nil {
			dataLogMu.Lock()
			dataLogFile.Close()
			dataLogMu.Unlock()
		}

		os.Exit(0)
	}()
}

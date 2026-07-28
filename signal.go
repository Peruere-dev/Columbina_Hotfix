package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func setupSignal() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		for {
			select {
			case s := <-sig:
				switch s {
				case syscall.SIGHUP:
					if err := reloadConfig(); err != nil {
						logError("SIGHUP reload config: " + err.Error())
					} else {
						reloadHotfixCache()
						logInfo("SIGHUP received: config + hotfix reloaded")
					}
				default:
					logInfo(L("shutting_down"))
					shutdown()
					return
				}
			case <-shutdownCh:
				logInfo(L("shutting_down"))
				shutdown()
				return
			}
		}
	}()
}

func shutdown() {
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
}
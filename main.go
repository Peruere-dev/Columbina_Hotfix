package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

var httpServer *http.Server

func main() {
	fmt.Println("ColumbinaHotfix")

	var err error
	configPath, err = findConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find/create config: %v\n", err)
		os.Exit(1)
	}

	if err := loadConfig(configPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// chdir to binary directory so relative paths (config, DB, keys) work
	if dir := filepath.Dir(configPath); dir != "." {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to chdir to %s: %v\n", dir, err)
		}
	}

	loadLang(configPath)

	fmt.Printf(L("config_loading"), configPath)
	fmt.Println()

	if err := initDB(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to init database: %v\n", err)
		os.Exit(1)
	}

	newUser, newPass, err := seedAdminConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to seed admin config: %v\n", err)
		os.Exit(1)
	}
	if newPass != "" {
		fmt.Printf("[ADMIN] 默认管理员 — 用户名: %s  密码: %s  面板: http://%s:%d%s\n",
			newUser, newPass,
			getConfig().Server.AccessAddress, getConfig().Server.AccessPort, getConfig().Admin.Route)
	}

	keysDir := findKeysDir()
	fmt.Printf(L("keys_loading"), keysDir)
	fmt.Println()
	if err := loadKeys(keysDir); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load keys: %v\n", err)
		os.Exit(1)
	}

	initLogging()
	initDataLogging()

	addr := fmt.Sprintf("%s:%d", getConfig().Server.BindAddress, getConfig().Server.BindPort)

	handler := NewDispatchServer()
	httpServer = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		schema := "http"
		listenFn := httpServer.ListenAndServe
		if getConfig().Server.TLS.Enable {
			schema = "https"
			listenFn = func() error {
				return httpServer.ListenAndServeTLS(getConfig().Server.TLS.CertFile, getConfig().Server.TLS.KeyFile)
			}
		}
		fmt.Printf(L("listening"), fmt.Sprintf("%s://%s", schema, addr))
		fmt.Println()
		writeLog(fmt.Sprintf(L("server_started"), fmt.Sprintf("%s://%s", schema, addr)))
		if err := listenFn(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, L("server_error"), err)
			fmt.Println()
			writeLog(fmt.Sprintf(L("server_error"), err))
			os.Exit(1)
		}
	}()

	// Wait for signal
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println(L("shutting_down"))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	httpServer.Shutdown(ctx)

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
}

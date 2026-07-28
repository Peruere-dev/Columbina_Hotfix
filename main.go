package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var httpServer *http.Server

func main() {
	startTime = time.Now()

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

	if dir := filepath.Dir(configPath); dir != "." {
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to chdir to %s: %v\n", dir, err)
		}
	}

	loadLang(configPath)
	initLogging()
	initDataLogging()
logInfo(L("config_loading", filepath.Base(configPath)))
	if err := initDB(); err != nil {
		logError("Failed to init database: " + err.Error())
		os.Exit(1)
	}

	newUser, newPass, err := seedAdminConfig()
	if err != nil {
		logError("Failed to seed admin config: " + err.Error())
		os.Exit(1)
	}
	if newPass != "" {
		adminURL := fmt.Sprintf("http://%s:%d%s",
			getConfig().Server.AccessAddress, getConfig().Server.AccessPort, getConfig().Admin.Route)
		logInfo(fmt.Sprintf("默认管理员 — 用户名: %s  密码: %s  面板: %s", newUser, newPass, adminURL))
	}

	keysDir := findKeysDir()
	logInfo(L("keys_loading", filepath.Base(keysDir)+"/"))
	if err := loadKeys(keysDir); err != nil {
		logError("Failed to load keys: " + err.Error())
		os.Exit(1)
	}

	showStartupBox()

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
		logInfo(fmt.Sprintf(L("listening"), fmt.Sprintf("%s://%s", schema, addr)))
		writeLog(fmt.Sprintf(L("server_started"), fmt.Sprintf("%s://%s", schema, addr)))
		if err := listenFn(); err != nil && err != http.ErrServerClosed {
			logError(fmt.Sprintf(L("server_error"), err))
			writeLog(fmt.Sprintf(L("server_error"), err))
			os.Exit(1)
		}
	}()

	startREPL()
	setupSignal()

	select {}
}

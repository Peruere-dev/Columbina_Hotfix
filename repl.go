package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

const (
	cReset = "\033[0m"
	cBlue  = "\033[34m"
	cCyan  = "\033[36m"
	cGreen = "\033[32m"
	cYellow = "\033[33m"
	cRed   = "\033[31m"
	cDim   = "\033[2m"
)

var (
	reqCount    atomic.Int64
	shutdownCh  = make(chan struct{})
	startTime   time.Time
)

func logInfo(msg string) {
	t := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s%s %s%s\n", cDim+t+cReset, cGreen+"INFO"+cReset, cReset, msg, cReset)
	writeLog(msg)
}

func logWarn(msg string) {
	t := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s%s %s%s\n", cDim+t+cReset, cYellow+"WARN"+cReset, cReset, msg, cReset)
	writeLog("[WARN] " + msg)
}

func logError(msg string) {
	t := time.Now().Format("15:04:05.000")
	fmt.Printf("%s %s%s %s%s\n", cDim+t+cReset, cRed+"ERROR"+cReset, cReset, msg, cReset)
	writeLog("[ERROR] " + msg)
}

func logRequest(method, path, ip string, status int, dur time.Duration) {
	t := time.Now().Format("15:04:05.000")
	ms := dur.Milliseconds()

	var mc string
	switch method {
	case "GET":    mc = cBlue
	case "POST":   mc = cCyan
	case "DELETE": mc = cRed
	case "PUT":    mc = cYellow
	default:       mc = cReset
	}

	var sc string
	switch {
	case status >= 500: sc = cRed
	case status >= 400: sc = cYellow
	case status >= 200: sc = cGreen
	default:            sc = cReset
	}

	var lc string
	switch {
	case ms > 1000: lc = cRed
	case ms > 500:  lc = cYellow
	default:        lc = cDim
	}

	lat := fmt.Sprintf("%dms", ms)
	if ms < 1 {
		lat = "<1ms"
	}

	fmt.Printf("%s %s %s %s %s%s %s\n",
		cDim+t+cReset,
		fmt.Sprintf("%s%3d%s", sc, status, cReset),
		fmt.Sprintf("%s%5s%s", lc, lat, cReset),
		cDim+ip+cReset,
		fmt.Sprintf("%s%-5s%s", mc, method, cReset),
		cReset, path,
	)
}

func showStartupBox() {
	cfg := getConfig()
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddress, cfg.Server.BindPort)
	admin := fmt.Sprintf("http://%s:%d%s", cfg.Server.AccessAddress, cfg.Server.AccessPort, cfg.Admin.Route)
	pid := fmt.Sprintf("%d", os.Getpid())

	inner := 48

	fmt.Printf("%s╭─ ColumbinaHotfix ─%s┐%s\n",
		cDim, strings.Repeat("─", inner-19), cReset)

	fmt.Printf("%s│%s  %sListen   %s%s%*s%s│%s\n",
		cDim, cReset, cBlue, cReset, addr,
		inner-2-9-len(addr), "", cDim, cReset)

	fmt.Printf("%s│%s  %sAdmin    %s%s%*s%s│%s\n",
		cDim, cReset, cBlue, cReset, admin,
		inner-2-9-len(admin), "", cDim, cReset)

	fmt.Printf("%s│%s  PID      %s%*s%s│%s\n",
		cDim, cReset, pid,
		inner-2-9-len(pid), "", cDim, cReset)

	fmt.Printf("%s╰%s╯%s\n",
		cDim, strings.Repeat("─", inner), cReset)
}

func startREPL() {
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for {
			fmt.Printf("%s>%s ", cBlue, cReset)
			if !scanner.Scan() {
				break
			}
			cmd := strings.TrimSpace(scanner.Text())
			switch cmd {
			case "help":
				printHelp()
			case "status":
				printStatus()
			case "stats":
				printStats()
			case "reload":
				doReload()
			case "stop":
				fmt.Println("  " + L("repl_stop_msg"))
				close(shutdownCh)
				return
			}
		}
	}()
}

func printHelp() {
	fmt.Println("  " + L("repl_help_help"))
	fmt.Println("  " + L("repl_help_status"))
	fmt.Println("  " + L("repl_help_stats"))
	fmt.Println("  " + L("repl_help_reload"))
	fmt.Println("  " + L("repl_help_stop"))
}

func printStatus() {
	uptime := time.Since(startTime)
	dash := getDashboardStats()
	fmt.Printf("  "+L("repl_status")+"\n",
		os.Getpid(), fmtDuration(uptime), reqCount.Load())
	fmt.Printf("  "+L("repl_status_users")+"\n",
		dash["total_users"], dash["active_users"], dash["banned_users"])
}

func printStats() {
	stats := getVersionStats()
	if len(stats) == 0 {
		fmt.Println("  " + L("repl_stats_empty"))
		return
	}
	for _, s := range stats {
		fmt.Printf("  "+L("repl_stats_line")+"\n", s.Version, s.Platform, s.RequestCount)
	}
}

func doReload() {
	if err := reloadConfig(); err != nil {
		logError("reload: " + err.Error())
		return
	}
	logInfo(L("repl_reload_ok"))
}

func fmtDuration(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

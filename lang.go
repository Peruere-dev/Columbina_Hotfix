package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	langStrings map[string]string
	langMu      sync.RWMutex
)

func loadLang(path string) {
	langFile := "lang_cn.json"
	if getConfig().Language == 1 {
		langFile = "lang_en.json"
	}
	dir := filepath.Dir(path)
	fullPath := filepath.Join(dir, "lang", langFile)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[LANG] failed to load %s: %v\n", fullPath, err)
		langStrings = make(map[string]string)
		return
	}
	m := make(map[string]string)
	if err := json.Unmarshal(data, &m); err != nil {
		fmt.Fprintf(os.Stderr, "[LANG] failed to parse %s: %v\n", fullPath, err)
		langStrings = make(map[string]string)
		return
	}
	langMu.Lock()
	langStrings = m
	langMu.Unlock()
	fmt.Printf("[LANG] loaded %s (%d entries)\n", langFile, len(m))
}

func L(key string, args ...interface{}) string {
	langMu.RLock()
	s, ok := langStrings[key]
	langMu.RUnlock()
	if !ok {
		return key
	}
	if len(args) > 0 {
		return fmt.Sprintf(s, args...)
	}
	return s
}

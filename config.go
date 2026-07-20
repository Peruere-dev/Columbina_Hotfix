package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	Server    ServerConfig     `json:"server"`
	GameServer GameServerConfig `json:"gameServer"`
	Admin     AdminConfig      `json:"admin"`
	Regions   []RegionConfig   `json:"regions"`
	Version   string           `json:"version"`
	LoadConfig struct {
		Name string `json:"name"`
	} `json:"loadConfig"`
	RegionType         string            `json:"regionType"`
	AutoCreateAccount  bool              `json:"autoCreateAccount"`
	UnsupportedVersion UnsupportedConfig `json:"unsupportedVersion"`
	StopServer         StopServerConfig  `json:"stopServer"`
	DataLog            DataLogConfig     `json:"dataLog"`
	Language           int               `json:"language"`
	EmailDomain        string            `json:"emailDomain"`
}

type ServerConfig struct {
	BindAddress      string    `json:"bindAddress"`
	BindPort         int       `json:"bindPort"`
	AccessAddress    string    `json:"accessAddress"`
	AccessPort       int       `json:"accessPort"`
	TLS              TLSConfig `json:"tls"`
	ForcePublicHTTPS bool      `json:"forcePublicHttps"`
}

type TLSConfig struct {
	Enable   bool   `json:"enable"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
}

type GameServerConfig struct {
	AccessAddress string `json:"accessAddress"`
	AccessPort    int    `json:"accessPort"`
}

type AdminConfig struct {
	Route string `json:"route"`
}

type RegionConfig struct {
	Name  string `json:"Name"`
	Title string `json:"Title"`
	Ip    string `json:"Ip"`
	Port  int    `json:"Port"`
}

type UnsupportedConfig struct {
	Message string `json:"message"`
	URL     string `json:"url"`
}

type DataLogConfig struct {
	Enable bool   `json:"enable"`
	Path   string `json:"path"`
}

type StopServerConfig struct {
	BeginTime uint32 `json:"beginTime"`
	EndTime   uint32 `json:"endTime"`
	URL       string `json:"url"`
	Message   string `json:"message"`
}

var cfg Config
var cfgMu sync.RWMutex
var configPath string

func getConfig() Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

func updateConfig(fn func(*Config)) error {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	fn(&cfg)
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			BindAddress:   "0.0.0.0",
			BindPort:      5200,
			AccessAddress: "127.0.0.1",
			AccessPort:    5200,
			TLS: TLSConfig{
				Enable:   false,
				CertFile: "cert.pem",
				KeyFile:  "key.pem",
			},
		},
		GameServer: GameServerConfig{
			AccessAddress: "127.0.0.1",
			AccessPort:    22101,
		},
		Admin: AdminConfig{
			Route: "/admin",
		},
		Regions: []RegionConfig{
			{Name: "hotfix", Title: "hotfix", Ip: "127.0.0.1", Port: 22101},
		},
		Version:   "6.5",
		RegionType: "DEV_XIYU",
		AutoCreateAccount: true,
		LoadConfig: struct {
			Name string `json:"name"`
		}{Name: "GI_Hotfix"},
		Language: 0,
		EmailDomain: "xiyu.com",
		DataLog: DataLogConfig{
			Enable: true,
			Path:   "data.log",
		},
		UnsupportedVersion: UnsupportedConfig{Message: "暂不支持当前版本"},
		StopServer: StopServerConfig{
			BeginTime: 0,
			EndTime:   0,
		},
	}
}

func saveDefaultConfig(path string) error {
	cfg = defaultConfig()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func findConfig() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	p := filepath.Join(dir, "config.json")
	if _, err := os.Stat(p); err == nil {
		return p, nil
	}
	fmt.Fprintf(os.Stderr, "config.json not found in binary directory, generating default\n")
	return p, saveDefaultConfig(p)
}

func findKeysDir() string {
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: cannot find executable path: %v\n", err)
		return "keys"
	}
	dir := filepath.Dir(exe)
	d := filepath.Join(dir, "keys")
	if _, err := os.Stat(filepath.Join(d, "dispatchKey.bin")); err == nil {
		return d
	}
	fmt.Fprintf(os.Stderr, "Warning: keys directory not found next to binary\n")
	return d
}

func loadConfig(path string) error {
	cfg = defaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &cfg)
}

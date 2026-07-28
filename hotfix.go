package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type HotfixData struct {
	ResourceURL    string            `json:"ResourceUrl"`
	DataURL        string            `json:"DataUrl"`
	DataVersion    uint32            `json:"ClientDataVersion,omitempty"`
	SilenceVersion uint32            `json:"ClientSilenceDataVersion,omitempty"`
	DataMD5        string            `json:"ClientDataMd5,omitempty"`
	SilenceMD5     string            `json:"ClientSilenceDataMd5,omitempty"`
	VersionSuffix  string            `json:"ClientVersionSuffix,omitempty"`
	SilenceSuffix  string            `json:"ClientSilenceVersionSuffix,omitempty"`
	ResVersion     *HotfixResVersion `json:"ResVersionConfig,omitempty"`
}

type HotfixResVersion struct {
	Version       uint32 `json:"Version,omitempty"`
	Md5           string `json:"Md5,omitempty"`
	ReleaseSize   string `json:"ReleaseTotalSize,omitempty"`
	VersionSuffix string `json:"VersionSuffix,omitempty"`
	Branch        string `json:"Branch,omitempty"`
}

var versionNumRe = regexp.MustCompile(`[^0-9.]`)

func extractVersionNum(s string) string {
	return versionNumRe.ReplaceAllString(s, "")
}

func regionPrefix(s string) string {
	if strings.HasPrefix(s, "OSREL") {
		return "OSREL"
	}
	return "CNREL"
}

func platformFromVersion(s string) string {
	if strings.Contains(s, "Win") || strings.Contains(s, "Windows") {
		return "Win"
	}
	if strings.Contains(s, "iOS") {
		return "iOS"
	}
	return "Android"
}

func hotfixPath(elem ...string) string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(append([]string{"hotfix"}, elem...)...)
	}
	return filepath.Join(append([]string{filepath.Dir(exe), "hotfix"}, elem...)...)
}

func LoadHotfixConfig(versionStr string) *HotfixData {
	if versionStr == "" {
		return nil
	}
	prefix := regionPrefix(versionStr)
	platform := platformFromVersion(versionStr)
	verNum := extractVersionNum(versionStr)
	if verNum == "" {
		return nil
	}

	path := hotfixPath(prefix, platform, verNum+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	var cfg HotfixData
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

func (h *HotfixData) BuildRegionInfoParams(ip string, port int, versionStr string) RegionInfoParams {
	areaType := "CN"
	if regionPrefix(versionStr) == "OSREL" {
		areaType = "OS"
	}

	// Derive bak fields from URLs
	resourceBak := extractBak(h.ResourceURL)
	dataBak := extractBak(h.DataURL)

	var resVer []byte
	if h.ResVersion != nil {
		resVer = BuildResVersionConfig(
			h.ResVersion.Version,
			false,
			h.ResVersion.Md5,
			h.ResVersion.ReleaseSize,
			h.ResVersion.VersionSuffix,
			h.ResVersion.Branch,
			0,
		)
	}

	return RegionInfoParams{
		GateserverIP:              ip,
		GateserverPort:            uint32(port),
		AreaType:                  areaType,
		ResourceURL:               h.ResourceURL,
		DataURL:                   h.DataURL,
		ResourceURLBak:            resourceBak,
		DataURLBak:                dataBak,
		ClientDataVersion:         h.DataVersion,
		ClientSilenceDataVersion:  h.SilenceVersion,
		ClientDataMD5:             h.DataMD5,
		ClientSilenceDataMD5:      h.SilenceMD5,
		ClientVersionSuffix:       h.VersionSuffix,
		ClientSilenceVersionSuffix: h.SilenceSuffix,
		ResVersionConfig:          resVer,
	}
}

func extractBak(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	idx := strings.LastIndex(rawURL, "/")
	if idx < 0 {
		return rawURL
	}
	return rawURL[idx+1:]
}

// GetHotfixPlatforms returns a list of available prefixes+platforms for the admin dashboard.
func GetHotfixPlatforms() []string {
	root := hotfixPath()
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			result = append(result, e.Name())
		}
	}
	return result
}

// GetHotfixVersions returns version strings available for a given prefix+platform.
func GetHotfixVersions(prefix, platform string) []string {
	dir := hotfixPath(prefix, platform)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			result = append(result, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return result
}

func init() {
	hotfixDir := hotfixPath()
	if _, err := os.Stat(hotfixDir); os.IsNotExist(err) {
		fmt.Printf("WARNING: hotfix directory not found at %s\n", hotfixDir)
	}
}

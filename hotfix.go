package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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

var (
	hotfixCache   map[string]*HotfixData
	hotfixCacheMu sync.RWMutex
)

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

func cacheKey(prefix, platform, verNum string) string {
	return prefix + "/" + platform + "/" + verNum
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

	key := cacheKey(prefix, platform, verNum)
	hotfixCacheMu.RLock()
	cfg, ok := hotfixCache[key]
	hotfixCacheMu.RUnlock()
	if ok {
		return cfg
	}

	path := hotfixPath(prefix, platform, verNum+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var hd HotfixData
	if err := json.Unmarshal(data, &hd); err != nil {
		return nil
	}
	return &hd
}

func initHotfixCache() {
	hotfixCacheMu.Lock()
	defer hotfixCacheMu.Unlock()
	hotfixCache = make(map[string]*HotfixData)

	root := hotfixPath()
	entries, err := os.ReadDir(root)
	if err != nil {
		logWarn(fmt.Sprintf("hotfix directory not found: %s", root))
		return
	}

	var count int
	for _, region := range entries {
		if !region.IsDir() {
			continue
		}
		rName := region.Name()
		platDir := hotfixPath(rName)
		plats, err := os.ReadDir(platDir)
		if err != nil {
			continue
		}
		for _, plat := range plats {
			if !plat.IsDir() {
				continue
			}
			pName := plat.Name()
			verDir := hotfixPath(rName, pName)
			vers, err := os.ReadDir(verDir)
			if err != nil {
				continue
			}
			for _, ver := range vers {
				if ver.IsDir() || !strings.HasSuffix(ver.Name(), ".json") {
					continue
				}
				verNum := strings.TrimSuffix(ver.Name(), ".json")
				path := hotfixPath(rName, pName, ver.Name())
				data, err := os.ReadFile(path)
				if err != nil {
					continue
				}
				var hd HotfixData
				if err := json.Unmarshal(data, &hd); err != nil {
					continue
				}
				key := cacheKey(rName, pName, verNum)
				hotfixCache[key] = &hd
				count++
			}
		}
	}
	logInfo(fmt.Sprintf("hotfix: cached %d configs", count))
}

func reloadHotfixCache() {
	if _, err := os.Stat(hotfixPath()); os.IsNotExist(err) {
		logWarn("hotfix directory removed, cache cleared")
		hotfixCacheMu.Lock()
		hotfixCache = make(map[string]*HotfixData)
		hotfixCacheMu.Unlock()
		return
	}
	initHotfixCache()
}

func (h *HotfixData) BuildRegionInfoParams(ip string, port int, versionStr string) RegionInfoParams {
	areaType := "CN"
	if regionPrefix(versionStr) == "OSREL" {
		areaType = "OS"
	}

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
		GateserverIP:               ip,
		GateserverPort:             uint32(port),
		AreaType:                   areaType,
		ResourceURL:                h.ResourceURL,
		DataURL:                    h.DataURL,
		ResourceURLBak:             resourceBak,
		DataURLBak:                 dataBak,
		ClientDataVersion:          h.DataVersion,
		ClientSilenceDataVersion:   h.SilenceVersion,
		ClientDataMD5:              h.DataMD5,
		ClientSilenceDataMD5:       h.SilenceMD5,
		ClientVersionSuffix:        h.VersionSuffix,
		ClientSilenceVersionSuffix: h.SilenceSuffix,
		ResVersionConfig:           resVer,
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
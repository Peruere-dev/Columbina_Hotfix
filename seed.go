package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type SeedMap map[string]map[string][]string

var (
	seedVerified  SeedMap
	seedCollected SeedMap
	seedDataValid bool
	seedBaseDir   string
	seedMu        sync.Mutex
)

const (
	collectedSeedFile = "dispatchseed_collected.json"
	verifiedSeedFile  = "dispatchseed_verified.json"
)

func initSeedCheck() {
	seedBaseDir = filepath.Dir(configPath)
	seedVerified = SeedMap{}
	seedCollected = SeedMap{}
	seedDataValid = true

	if !getConfig().SeedCheck.Enabled {
		return
	}

	if m, err := loadSeedMap(filepath.Join(seedBaseDir, verifiedSeedFile)); err != nil {
		seedDataValid = false
		logError(fmt.Sprintf("seed check: verified file unusable (%v); falling back to no verification and no writes", err))
	} else {
		seedVerified = m
		ensureSeedFile(filepath.Join(seedBaseDir, verifiedSeedFile), m)
	}
	if m, err := loadSeedMap(filepath.Join(seedBaseDir, collectedSeedFile)); err != nil {
		seedDataValid = false
		logError(fmt.Sprintf("seed check: collected file unusable (%v); falling back to no verification and no writes", err))
	} else {
		seedCollected = m
		ensureSeedFile(filepath.Join(seedBaseDir, collectedSeedFile), m)
	}
}

func ensureSeedFile(path string, m SeedMap) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := writeSeedMapAtomic(path, m); err != nil {
			logError(fmt.Sprintf("seed check: failed to create %s: %v", filepath.Base(path), err))
		}
	}
}

func loadSeedMap(path string) (SeedMap, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return SeedMap{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := SeedMap{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

func seedWriteEnabled() bool {
	return getConfig().SeedCheck.Enabled && seedDataValid
}

func isValidHex16(s string) bool {
	if len(s) != 16 {
		return false
	}
	for _, c := range s {
		switch {
		case c >= '0' && c <= '9', c >= 'a' && c <= 'f', c >= 'A' && c <= 'F':
		default:
			return false
		}
	}
	return true
}

var seedPlatformWords = []string{"Windows", "Android", "iOS", "Win"}

// canonicalVersion strips the platform word from a full version string,
// e.g. "CNRELAndroid6.7.0" -> "CNREL6.7.0".
func canonicalVersion(v string) string {
	out := v
	for _, w := range seedPlatformWords {
		out = strings.ReplaceAll(out, w, "")
	}
	return out
}

// platformName maps the numeric platform query param to a name.
// Falls back to parsing the version string when param is empty/unknown.
func platformName(platform, version string) string {
	switch platform {
	case "1":
		return "iOS"
	case "2":
		return "Android"
	case "3":
		return "Win"
	}
	if strings.Contains(version, "Windows") || strings.Contains(version, "Win") {
		return "Win"
	}
	if strings.Contains(version, "iOS") {
		return "iOS"
	}
	if strings.Contains(version, "Android") {
		return "Android"
	}
	return ""
}

// seedReject reports whether the dispatchSeed on a query_cur_region request must be rejected.
func seedReject(version, platform, dispatchSeed, regionSuffix string) bool {
	if !seedWriteEnabled() {
		return false
	}

	vKey := canonicalVersion(version)
	pKey := platformName(platform, version)

	if seeds, ok := lookupSeed(seedVerified, vKey, pKey); ok {
		if dispatchSeed == "" {
			return true
		}
		for _, s := range seeds {
			if s == dispatchSeed {
				return false
			}
		}
		return true
	}

	if isValidHex16(dispatchSeed) {
		recordCollectedSeed(vKey, pKey, dispatchSeed)
		return false
	}
	if isProbePath(regionSuffix) {
		return false
	}
	return true
}

func lookupSeed(m SeedMap, version, platform string) ([]string, bool) {
	pf, ok := m[version]
	if !ok {
		return nil, false
	}
	seeds, ok := pf[platform]
	return seeds, ok && len(seeds) > 0
}

func isProbePath(suffix string) bool {
	if suffix == "" {
		return false
	}
	for _, r := range getConfig().Regions {
		if r.Name == suffix {
			return true
		}
	}
	return false
}

func recordCollectedSeed(version, platform, seed string) {
	if version == "" || platform == "" {
		return
	}
	seedMu.Lock()
	defer seedMu.Unlock()
	pf, ok := seedCollected[version]
	if !ok {
		pf = map[string][]string{}
		seedCollected[version] = pf
	}
	for _, s := range pf[platform] {
		if s == seed {
			return
		}
	}
	pf[platform] = append(pf[platform], seed)
	if err := writeSeedMapAtomic(filepath.Join(seedBaseDir, collectedSeedFile), seedCollected); err != nil {
		logError(fmt.Sprintf("seed check: failed to persist collected file: %v", err))
	}
}

func writeSeedMapAtomic(path string, m SeedMap) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

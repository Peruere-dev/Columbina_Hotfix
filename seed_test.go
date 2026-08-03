package main

import (
	"crypto/rand"
	"crypto/rsa"
	"os"
	"path/filepath"
	"testing"
)

func TestIsValidHex16(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"aaaaaaaaaaaaaaaa", true},
		{"E00FA3521AF2486E", true},
		{"0123456789abcdef", true},
		{"", false},
		{"e00fa3521af2486", false},
		{"aaaaaaaaaaaaaaaaf", false},
		{"zzzzzzzzzzzzzzzz", false},
		{"e00fa3521af2486g", false},
	}
	for _, c := range cases {
		if got := isValidHex16(c.in); got != c.want {
			t.Errorf("isValidHex16(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestCanonicalKeys(t *testing.T) {
	cases := []struct {
		version string
		platform string
		wantVer string
		wantPlat string
	}{
		{"CNRELAndroid6.7.0", "2", "CNREL6.7.0", "Android"},
		{"CNRELWin6.7.0", "3", "CNREL6.7.0", "Win"},
		{"CNRELiOS6.7.0", "1", "CNREL6.7.0", "iOS"},
		{"CNRELWindows6.7.0", "3", "CNREL6.7.0", "Win"},
		{"OSRELWin6.6.0", "3", "OSREL6.6.0", "Win"},
		{"CNREL6.7.0", "", "CNREL6.7.0", ""},
	}
	for _, c := range cases {
		if got := canonicalVersion(c.version); got != c.wantVer {
			t.Errorf("canonicalVersion(%q) = %q, want %q", c.version, got, c.wantVer)
		}
		if got := platformName(c.platform, c.version); got != c.wantPlat {
			t.Errorf("platformName(%q, %q) = %q, want %q", c.platform, c.version, got, c.wantPlat)
		}
	}
}

func TestSeedReject(t *testing.T) {
	seedBaseDir = t.TempDir()
	cfg = Config{
		SeedCheck: SeedCheckConfig{Enabled: true},
		Regions: []RegionConfig{
			{Name: "hotfix"},
			{Name: "gio6.7.0"},
		},
	}
	seedVerified = SeedMap{
		"CNREL6.7.0": {"Android": []string{"aaaaaaaaaaaaaaaa"}},
	}
	seedCollected = SeedMap{}
	seedDataValid = true

	if seedReject("CNRELAndroid6.7.0", "2", "aaaaaaaaaaaaaaaa", "hotfix") {
		t.Error("verified match should pass")
	}
	if !seedReject("CNRELAndroid6.7.0", "2", "cccccccccccccccc", "hotfix") {
		t.Error("verified mismatch should reject")
	}
	if !seedReject("CNRELAndroid6.7.0", "2", "", "hotfix") {
		t.Error("verified missing seed should reject")
	}

	seedCollected = SeedMap{}
	if seedReject("CNRELWin6.7.0", "3", "bbbbbbbbbbbbbbbb", "hotfix") {
		t.Error("unverified 16hex should pass")
	}
	if _, ok := lookupSeed(seedCollected, "CNREL6.7.0", "Win"); !ok {
		t.Error("16hex seed should be collected under canonical key CNREL6.7.0/Win")
	}
	if _, err := os.Stat(filepath.Join(seedBaseDir, collectedSeedFile)); err != nil {
		t.Error("collected file should be written:", err)
	}

	if seedReject("CNRELWin6.7.0", "3", "zzzz", "hotfix") {
		t.Error("bad seed on whitelist path should pass")
	}
	if !seedReject("CNRELWin6.7.0", "3", "zzzz", "evil") {
		t.Error("bad seed on non-whitelist path should reject")
	}

	cfg.SeedCheck.Enabled = false
	if seedReject("CNRELWin6.7.0", "3", "zzzz", "evil") {
		t.Error("test mode should never reject")
	}
	cfg.SeedCheck.Enabled = true

	seedDataValid = false
	if seedReject("CNRELWin6.7.0", "3", "zzzz", "evil") {
		t.Error("corrupt-file fallback should not reject")
	}
}

func TestEncryptAndSignKeyID(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	encryptionKeys = map[int]*rsa.PublicKey{4: &priv.PublicKey}
	signingKey = priv

	if r := encryptAndSignRegionData([]byte("hello"), "4"); r["content"] == "" {
		t.Error("key_id 4 should encrypt")
	}
	for _, kid := range []string{"", "0", "999", "abc"} {
		if r := encryptAndSignRegionData([]byte("hello"), kid); r["content"] != "" {
			t.Errorf("key_id %q should fall back to plaintext", kid)
		}
	}
}

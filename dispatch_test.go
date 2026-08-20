package main

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func TestRegionFromPath(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/query_cur_region/hotfix", "hotfix"},
		{"/query_gateserver/hotfix", "hotfix"},
		{"/query_cur_region/gio6.7.0", "gio6.7.0"},
		{"/query_gateserver/gio6.7.0", "gio6.7.0"},
		{"/query_cur_region", ""},
		{"/query_gateserver", ""},
		{"/query_cur_region/", ""},
		{"/", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := regionFromPath(c.path); got != c.want {
			t.Errorf("regionFromPath(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestSdkEnv(t *testing.T) {
	cases := []struct {
		version string
		want    string
	}{
		{"CNRELAndroid6.7.0", "0"},
		{"CNRELWin6.7.0", "0"},
		{"CNRELiOS6.7.0", "0"},
		{"CNDevAndroid7.0.0", "0"},
		{"CNCB2.8.50", "0"},
		{"CNREL", "0"},
		{"OSRELWin7.0.0", "2"},
		{"OSRELAndroid6.5.1", "2"},
		{"GCPS3.5.0", "2"},
		{"", "2"},
		{"C", "2"},
	}
	for _, c := range cases {
		if got := sdkEnv(c.version); got != c.want {
			t.Errorf("sdkEnv(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}

func TestEffectiveBan(t *testing.T) {
	now := time.Now()
	future := now.Add(24 * time.Hour).Format("2006-01-02 15:04:05")
	past := now.Add(-24 * time.Hour).Format("2006-01-02 15:04:05")

	perm := &Account{Banned: 1, BanReason: "r", BanPermanent: 1}
	if b, r, p, _ := effectiveBan(perm); !b || !p || r != "r" {
		t.Error("permanent ban must stay banned")
	}

	active := &Account{Banned: 1, BanReason: "r", BanExpiresAt: future}
	if b, _, p, ex := effectiveBan(active); !b || p || ex == "" {
		t.Errorf("future timed ban should be banned: banned=%v perm=%v expires=%q", b, p, ex)
	}

	expired := &Account{Banned: 1, BanReason: "r", BanExpiresAt: past}
	if b, _, _, _ := effectiveBan(expired); b {
		t.Error("expired timed ban must be treated as not banned")
	}

	none := &Account{}
	if b, _, _, _ := effectiveBan(none); b {
		t.Error("non-banned account must not be banned")
	}
}

func TestRegionRespCache(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	encryptionKeys = map[int]*rsa.PublicKey{4: &priv.PublicKey}
	signingKey = priv
	clearRegionRespCache()

	first := encryptAndSignRegionData([]byte("hello"), "4")
	if first["content"] == "" {
		t.Fatal("key_id 4 should encrypt")
	}
	second := encryptAndSignRegionData([]byte("hello"), "4")
	if first["content"] != second["content"] || first["sign"] != second["sign"] {
		t.Error("identical plaintext + key_id should reuse the cached encrypted response")
	}

	other := encryptAndSignRegionData([]byte("hello world"), "4")
	if other["content"] == first["content"] {
		t.Error("different plaintext must produce a different cached entry")
	}

	clearRegionRespCache()
	after := encryptAndSignRegionData([]byte("hello"), "4")
	if after["content"] == "" {
		t.Error("re-encryption after cache clear must stay valid")
	}
}

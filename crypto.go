package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

var (
	dispatchKey    []byte
	dispatchSeed   []byte
	signingKey     *rsa.PrivateKey
	encryptionKeys map[int]*rsa.PublicKey
)

func loadKeys(keysDir string) error {
	dispatchKey = readFileOrPanic(filepath.Join(keysDir, "dispatchKey.bin"))
	dispatchSeed = readFileOrPanic(filepath.Join(keysDir, "dispatchSeed.bin"))

	signDer := readFileOrPanic(filepath.Join(keysDir, "SigningKey.der"))
	key, err := x509.ParsePKCS8PrivateKey(signDer)
	if err != nil {
		return fmt.Errorf("parse SigningKey.der: %w", err)
	}
	var ok bool
	signingKey, ok = key.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("SigningKey.der is not RSA")
	}

	encryptionKeys = make(map[int]*rsa.PublicKey)
	pattern := regexp.MustCompile(`(\d+)_Pub\.der`)
	entries, err := os.ReadDir(filepath.Join(keysDir, "game_keys"))
	if err != nil {
		return fmt.Errorf("read game_keys: %w", err)
	}
	for _, e := range entries {
		m := pattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		var id int
		fmt.Sscanf(m[1], "%d", &id)
		pubDer := readFileOrPanic(filepath.Join(keysDir, "game_keys", e.Name()))
		pub, err := x509.ParsePKIXPublicKey(pubDer)
		if err != nil {
			return fmt.Errorf("parse %s: %w", e.Name(), err)
		}
		rsaPub, ok := pub.(*rsa.PublicKey)
		if !ok {
			return fmt.Errorf("%s is not RSA", e.Name())
		}
		encryptionKeys[id] = rsaPub
	}
	return nil
}

func readFileOrPanic(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Sprintf("failed to read %s: %v", path, err))
	}
	return data
}

func xorEncrypt(data, key []byte) []byte {
	if len(key) == 0 {
		return data
	}
	res := make([]byte, len(data))
	for i := range data {
		res[i] = data[i] ^ key[i%len(key)]
	}
	return res
}

func encryptAndSignRegionData(regionInfo []byte, keyID string) map[string]string {
	kid := 0
	fmt.Sscanf(keyID, "%d", &kid)
	pubKey, ok := encryptionKeys[kid]
	if !ok {
		ids := make([]int, 0, len(encryptionKeys))
		for id := range encryptionKeys {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		if len(ids) > 0 {
			pubKey = encryptionKeys[ids[0]]
		}
	}
	if pubKey == nil {
		return map[string]string{"content": "", "sign": ""}
	}

	chunkSize := 256 - 11
	var encrypted []byte
	for i := 0; i < len(regionInfo); i += chunkSize {
		end := i + chunkSize
		if end > len(regionInfo) {
			end = len(regionInfo)
		}
		chunk, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, regionInfo[i:end])
		if err != nil {
			return map[string]string{"content": "", "sign": ""}
		}
		encrypted = append(encrypted, chunk...)
	}

	hashed := sha256.Sum256(regionInfo)
	sig, err := rsa.SignPKCS1v15(rand.Reader, signingKey, crypto.SHA256, hashed[:])
	if err != nil {
		return map[string]string{"content": base64.StdEncoding.EncodeToString(encrypted), "sign": ""}
	}

	return map[string]string{
		"content": base64.StdEncoding.EncodeToString(encrypted),
		"sign":    base64.StdEncoding.EncodeToString(sig),
	}
}

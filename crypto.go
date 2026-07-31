package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"embed"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

//go:embed keys/dispatchKey.bin
var embedDispatchKey []byte

//go:embed keys/dispatchSeed.bin
var embedDispatchSeed []byte

//go:embed keys/SigningKey.der
var embedSigningKey []byte

//go:embed keys/game_keys
var embedGameKeys embed.FS

var (
	dispatchKey    []byte
	dispatchSeed   []byte
	signingKey     *rsa.PrivateKey
	encryptionKeys map[int]*rsa.PublicKey
)

func loadKeys() error {
	dispatchKey = readKey("dispatchKey.bin", embedDispatchKey)
	dispatchSeed = readKey("dispatchSeed.bin", embedDispatchSeed)

	signDer := readKey("SigningKey.der", embedSigningKey)
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

	gameEntries, err := embedGameKeys.ReadDir("keys/game_keys")
	if err != nil {
		return fmt.Errorf("read embedded game_keys: %w", err)
	}
	for _, e := range gameEntries {
		m := pattern.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		var id int
		fmt.Sscanf(m[1], "%d", &id)

		pubDer, err := readGameKey(e.Name())
		if err != nil {
			return fmt.Errorf("read %s: %w", e.Name(), err)
		}
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
	if _, ok := encryptionKeys[4]; !ok {
		writeLog("[WARN] game_keys: 4_Pub.der not found, some clients may fail")
	}
	return nil
}

func readKey(name string, embedData []byte) []byte {
	if data, err := os.ReadFile(filepath.Join("keys", name)); err == nil {
		return data
	}
	return embedData
}

func readGameKey(name string) ([]byte, error) {
	diskPath := filepath.Join("keys", "game_keys", name)
	if data, err := os.ReadFile(diskPath); err == nil {
		return data, nil
	}
	return embedGameKeys.ReadFile("keys/game_keys/" + name)
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
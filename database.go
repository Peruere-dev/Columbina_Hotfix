package main

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var dbLog = log.New(os.Stderr, "[DB] ", log.Ltime|log.Ldate)

type Account struct {
	ID              int
	UID             string
	Username        string
	Password        string
	Email           string
	Token           string
	SessionKey      string
	LoginToken      string
	CreatedAt       string
	LastLogin       string
	DeviceID        string
	TokenExpiresAt  string
	Banned          int
	BanReason       string
	BanExpiresAt    string
	BanPermanent    int
}

var (
	db       *sql.DB
	insertMu sync.Mutex
)

func initDB() error {
	var err error
	db, err = sql.Open("sqlite", "file:genshin.db?cache=shared&_pragma=busy_timeout(5000)")
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return fmt.Errorf("ping db: %w", err)
	}
	return migrateDB()
}

func migrateDB() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS accounts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uid TEXT UNIQUE NOT NULL,
			username TEXT UNIQUE NOT NULL,
			password TEXT NOT NULL DEFAULT '',
			email TEXT NOT NULL DEFAULT '',
			token TEXT NOT NULL DEFAULT '',
			session_key TEXT NOT NULL DEFAULT '',
			login_token TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			last_login TIMESTAMP,
			device_id TEXT NOT NULL DEFAULT '',
			token_expires_at TIMESTAMP,
			banned INTEGER NOT NULL DEFAULT 0,
			ban_reason TEXT NOT NULL DEFAULT '',
			ban_expires_at TIMESTAMP,
			ban_permanent INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_uid ON accounts(uid)`,
		`CREATE INDEX IF NOT EXISTS idx_username ON accounts(username)`,
		`CREATE INDEX IF NOT EXISTS idx_token ON accounts(token)`,
		`CREATE TABLE IF NOT EXISTS stats (
			id INTEGER PRIMARY KEY DEFAULT 1,
			hot_update_count INTEGER NOT NULL DEFAULT 0
		)`,
		`INSERT OR IGNORE INTO stats (id, hot_update_count) VALUES (1, 0)`,
		`CREATE TABLE IF NOT EXISTS version_stats (
			version TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT '',
			request_count INTEGER NOT NULL DEFAULT 0,
			last_request TIMESTAMP,
			PRIMARY KEY (version, platform)
		)`,
		`CREATE TABLE IF NOT EXISTS admin_config (
			id INTEGER PRIMARY KEY DEFAULT 1,
			username TEXT NOT NULL,
			password_hash TEXT NOT NULL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("exec: %w", err)
		}
	}
	return nil
}

func scanAccount(row *sql.Row) (*Account, error) {
	a := &Account{}
	var lastLogin, tokenExpiresAt, banExpiresAt sql.NullString
	err := row.Scan(&a.ID, &a.UID, &a.Username, &a.Password, &a.Email, &a.Token, &a.SessionKey, &a.LoginToken,
		&a.CreatedAt, &lastLogin, &a.DeviceID, &tokenExpiresAt, &a.Banned, &a.BanReason, &banExpiresAt, &a.BanPermanent)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a.LastLogin = lastLogin.String
	a.TokenExpiresAt = tokenExpiresAt.String
	a.BanExpiresAt = banExpiresAt.String
	return a, nil
}

func getAccountByUsername(username string) (*Account, error) {
	return scanAccount(db.QueryRow("SELECT id, uid, username, password, email, token, session_key, login_token, created_at, last_login, device_id, token_expires_at, banned, ban_reason, ban_expires_at, ban_permanent FROM accounts WHERE username = ?", username))
}

func getAccountByUID(uid string) (*Account, error) {
	return scanAccount(db.QueryRow("SELECT id, uid, username, password, email, token, session_key, login_token, created_at, last_login, device_id, token_expires_at, banned, ban_reason, ban_expires_at, ban_permanent FROM accounts WHERE uid = ?", uid))
}

func getNextUID() string {
	var maxUID int
	err := db.QueryRow("SELECT COALESCE(MAX(CAST(uid AS INTEGER)), 0) FROM accounts WHERE uid GLOB '[0-9]*'").Scan(&maxUID)
	if err != nil {
		return "10000001"
	}
	next := 10000001
	if maxUID >= next {
		next = maxUID + 1
	}
	return fmt.Sprintf("%d", next)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(h[:]), nil
}

func checkPassword(password, hash string) bool {
	parts := split2(hash, ":")
	if len(parts) != 2 {
		return false
	}
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}
	h := sha256.Sum256(append(salt, []byte(password)...))
	return hex.EncodeToString(h[:]) == parts[1]
}

func split2(s, sep string) []string {
	idx := strings.Index(s, sep)
	if idx < 0 {
		return []string{s}
	}
	return []string{s[:idx], s[idx+len(sep):]}
}

func createAccount(username, password string) (*Account, error) {
	insertMu.Lock()
	defer insertMu.Unlock()

	exists, err := getAccountByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("check existing: %w", err)
	}
	if exists != nil {
		return exists, nil
	}

	hashed, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("hash: %w", err)
	}

	uid := getNextUID()
	sessionKey := generateToken(32)
	_, err = db.Exec("INSERT INTO accounts (uid, username, password, email, token, session_key) VALUES (?, ?, ?, '', '', ?)",
		uid, username, hashed, sessionKey)
	if err != nil {
		return nil, fmt.Errorf("insert: %w", err)
	}
	return getAccountByUID(uid)
}

func updateSessionKey(uid, deviceID string) string {
	sessionKey := generateToken(32)
	expiresAt := time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.Exec("UPDATE accounts SET session_key = ?, last_login = CURRENT_TIMESTAMP, token_expires_at = ?, device_id = ? WHERE uid = ?",
		sessionKey, expiresAt, deviceID, uid); err != nil {
		dbLog.Printf("updateSessionKey(%s): %v", uid, err)
	}
	return sessionKey
}

func isTokenExpired(uid string) bool {
	var v string
	err := db.QueryRow("SELECT COALESCE(token_expires_at, '') FROM accounts WHERE uid = ?", uid).Scan(&v)
	if err != nil || v == "" {
		return true
	}
	t, err := time.Parse("2006-01-02 15:04:05", v)
	if err != nil {
		return true
	}
	return time.Now().After(t)
}

func extendTokenExpiry(uid string) {
	expiresAt := time.Now().Add(7 * 24 * time.Hour).Format("2006-01-02 15:04:05")
	if _, err := db.Exec("UPDATE accounts SET token_expires_at = ? WHERE uid = ?", expiresAt, uid); err != nil {
		dbLog.Printf("extendTokenExpiry(%s): %v", uid, err)
	}
}

func updateAccountLoginToken(username string) string {
	loginToken := generateToken(32)
	if _, err := db.Exec("UPDATE accounts SET login_token = ? WHERE username = ?", loginToken, username); err != nil {
		dbLog.Printf("updateAccountLoginToken(%s): %v", username, err)
	}
	return loginToken
}

func updateUserPassword(username, newPassword string) error {
	hashed, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = db.Exec("UPDATE accounts SET password = ? WHERE username = ?", hashed, username)
	return err
}

func isUserBannedInfo(username string) (banned bool, reason string, permanent bool, expiresAt string) {
	row := db.QueryRow("SELECT banned, ban_reason, ban_expires_at, ban_permanent FROM accounts WHERE username = ?", username)
	var b, bp int
	var br, bea sql.NullString
	if err := row.Scan(&b, &br, &bea, &bp); err != nil {
		return false, "", false, ""
	}
	if bp == 1 {
		return true, br.String, true, ""
	}
	if bea.Valid && bea.String != "" {
		t, err := time.Parse("2006-01-02 15:04:05", bea.String)
		if err != nil {
			t, err = time.Parse(time.RFC3339, bea.String)
		}
		if err == nil && time.Now().Before(t) {
			return true, br.String, false, t.Format("2006-01-02 15:04:05")
		}
		}
	return false, "", false, ""
}

func getDashboardStats() map[string]int {
	var total, banned int
	db.QueryRow("SELECT COUNT(*) FROM accounts").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM accounts WHERE banned = 1 OR ban_permanent = 1").Scan(&banned)
	return map[string]int{
		"total_users":  total,
		"active_users": total - banned,
		"banned_users": banned,
	}
}

func getAllAccounts() ([]*Account, error) {
	rows, err := db.Query("SELECT id, uid, username, password, email, token, session_key, login_token, created_at, last_login, device_id, token_expires_at, banned, ban_reason, ban_expires_at, ban_permanent FROM accounts ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var accounts []*Account
	for rows.Next() {
		a := &Account{}
		var lastLogin, tokenExpiresAt, banExpiresAt sql.NullString
		if err := rows.Scan(&a.ID, &a.UID, &a.Username, &a.Password, &a.Email, &a.Token, &a.SessionKey, &a.LoginToken,
			&a.CreatedAt, &lastLogin, &a.DeviceID, &tokenExpiresAt, &a.Banned, &a.BanReason, &banExpiresAt, &a.BanPermanent); err != nil {
			return nil, err
		}
		a.LastLogin = lastLogin.String
		a.TokenExpiresAt = tokenExpiresAt.String
		a.BanExpiresAt = banExpiresAt.String
		accounts = append(accounts, a)
	}
	return accounts, nil
}

func deleteAccount(username string) error {
	_, err := db.Exec("DELETE FROM accounts WHERE username = ?", username)
	return err
}

func updateUID(username, newUID string) error {
	_, err := db.Exec("UPDATE accounts SET uid = ? WHERE username = ?", newUID, username)
	return err
}

func updateBan(username, reason string, permanent bool, expiresAt string) error {
	if permanent {
		_, err := db.Exec("UPDATE accounts SET banned = 1, ban_reason = ?, ban_permanent = 1, ban_expires_at = NULL, session_key = '', token = '', login_token = '' WHERE username = ?", reason, username)
		return err
	}
	_, err := db.Exec("UPDATE accounts SET banned = 1, ban_reason = ?, ban_permanent = 0, ban_expires_at = ?, session_key = '', token = '', login_token = '' WHERE username = ?", reason, expiresAt, username)
	return err
}

func unbanUser(username string) error {
	_, err := db.Exec("UPDATE accounts SET banned = 0, ban_reason = '', ban_permanent = 0, ban_expires_at = NULL WHERE username = ?", username)
	return err
}

func renameUser(username, newUsername string) error {
	_, err := db.Exec("UPDATE accounts SET username = ? WHERE username = ?", newUsername, username)
	return err
}

func resetUserToken(username string) error {
	_, err := db.Exec("UPDATE accounts SET session_key = '', token = '', login_token = '' WHERE username = ?", username)
	return err
}

func getHotUpdateCount() int {
	var count int
	db.QueryRow("SELECT hot_update_count FROM stats WHERE id = 1").Scan(&count)
	return count
}

func incrementHotUpdateCount() {
	if _, err := db.Exec("UPDATE stats SET hot_update_count = hot_update_count + 1 WHERE id = 1"); err != nil {
		dbLog.Printf("incrementHotUpdateCount: %v", err)
	}
}

func recordVersionRequest(version, platform string) {
	if version == "" {
		return
	}
	insertMu.Lock()
	defer insertMu.Unlock()
	if _, err := db.Exec(`INSERT INTO version_stats (version, platform, request_count, last_request) VALUES (?, ?, 1, CURRENT_TIMESTAMP)
		ON CONFLICT(version, platform) DO UPDATE SET request_count = request_count + 1, last_request = CURRENT_TIMESTAMP`, version, platform); err != nil {
		dbLog.Printf("recordVersionRequest(%s, %s): %v", version, platform, err)
	}
}

type VersionStat struct {
	Version      string `json:"version"`
	Platform     string `json:"platform"`
	RequestCount int    `json:"request_count"`
	LastRequest  string `json:"last_request"`
}

func getVersionStats() []VersionStat {
	rows, err := db.Query("SELECT version, platform, request_count, COALESCE(last_request, '') FROM version_stats ORDER BY version, platform")
	if err != nil {
		return nil
	}
	defer rows.Close()
	var stats []VersionStat
	for rows.Next() {
		var s VersionStat
		if err := rows.Scan(&s.Version, &s.Platform, &s.RequestCount, &s.LastRequest); err != nil {
			dbLog.Printf("getVersionStats scan: %v", err)
			continue
		}
		stats = append(stats, s)
	}
	return stats
}

func generateToken(length int) string {
	b := make([]byte, length/2)
	if _, err := rand.Read(b); err != nil {
		panic("Failed to generate random token: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// Admin sessions (in-memory)
var (
	adminSessionsMu sync.RWMutex
	adminSessions   = map[string]adminSession{}
)

type adminSession struct {
	username  string
	expiresAt time.Time
}

func createAdminSession(username string) string {
	token := generateToken(32)
	adminSessionsMu.Lock()
	adminSessions[token] = adminSession{username: username, expiresAt: time.Now().Add(24 * time.Hour)}
	adminSessionsMu.Unlock()
	return token
}

func validateAdminSession(token string) string {
	adminSessionsMu.Lock()
	defer adminSessionsMu.Unlock()
	s, ok := adminSessions[token]
	if !ok || time.Now().After(s.expiresAt) {
		if ok {
			delete(adminSessions, token)
		}
		return ""
	}
	return s.username
}

func revokeAdminSession(token string) {
	adminSessionsMu.Lock()
	delete(adminSessions, token)
	adminSessionsMu.Unlock()
}

type AdminCredentials struct {
	Username     string
	PasswordHash string
}

func getAdminCredentials() (*AdminCredentials, error) {
	row := db.QueryRow("SELECT username, password_hash FROM admin_config WHERE id = 1")
	var a AdminCredentials
	if err := row.Scan(&a.Username, &a.PasswordHash); err != nil {
		return nil, err
	}
	return &a, nil
}

func updateAdminCredentials(username, passwordHash string) error {
	_, err := db.Exec("UPDATE admin_config SET username = ?, password_hash = ? WHERE id = 1", username, passwordHash)
	return err
}

func seedAdminConfig() (username, password string, err error) {
	existing, _ := getAdminCredentials()
	if existing != nil {
		return "", "", nil
	}
	username = "admin"
	password = "123456"
	hashed, hashErr := hashPassword(password)
	if hashErr != nil {
		return "", "", hashErr
	}
	_, err = db.Exec("INSERT INTO admin_config (id, username, password_hash) VALUES (1, ?, ?)", username, hashed)
	if err != nil {
		return "", "", err
	}
	return username, password, nil
}

func resetAdminCredentials(username, password string) error {
	hashed, err := hashPassword(password)
	if err != nil {
		return err
	}
	_, err = db.Exec("INSERT OR REPLACE INTO admin_config (id, username, password_hash) VALUES (1, ?, ?)", username, hashed)
	return err
}

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	logger      *log.Logger
	logFile     *os.File
	logMu       sync.Mutex
	dataLogger  *log.Logger
	dataLogFile *os.File
	dataLogMu   sync.Mutex
)

func initLogging() {
	var err error
	logFile, err = os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	logger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.Ltime|log.Ldate)
}

func initDataLogging() {
	if !getConfig().DataLog.Enable {
		return
	}
	var err error
	dataLogFile, err = os.OpenFile(getConfig().DataLog.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		writeLog(fmt.Sprintf(L("data_log_warn"), getConfig().DataLog.Path, err))
		return
	}
	dataLogger = log.New(dataLogFile, "", log.Ltime|log.Ldate)
	writeLog(fmt.Sprintf(L("data_log_init"), getConfig().DataLog.Path))
}

const maxLogSize = 10 << 20

func rotateLog(path string) {
	fi, err := os.Stat(path)
	if err != nil || fi.Size() < maxLogSize {
		return
	}
	os.Rename(path, path+".old")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return
	}
	if path == "server.log" {
		logFile = f
		logger = log.New(io.MultiWriter(os.Stdout, logFile), "", log.Ltime|log.Ldate)
	} else if path == getConfig().DataLog.Path {
		dataLogFile = f
		dataLogger = log.New(dataLogFile, "", log.Ltime|log.Ldate)
	}
}

func writeLog(msg string) {
	logMu.Lock()
	rotateLog("server.log")
	logger.Println(msg)
	logMu.Unlock()
}

func writeDataLog(msg string) {
	if !getConfig().DataLog.Enable || dataLogger == nil {
		return
	}
	dataLogMu.Lock()
	rotateLog(getConfig().DataLog.Path)
	dataLogger.Println(msg)
	dataLogMu.Unlock()
}

type DispatchServer struct{}

func NewDispatchServer() *DispatchServer {
	return &DispatchServer{}
}

func (s *DispatchServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeLog(fmt.Sprintf("[ERROR] read body: %v", err))
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if r.URL.Path == "/log/sdk/upload" || r.URL.Path == "/sdk/upload" || r.URL.Path == "/sdk/dataUpload" ||
		r.URL.Path == "/log" || r.URL.Path == "/crash/dataUpload" || r.URL.Path == "/perf/config/verify" {
		writeDataLog(fmt.Sprintf("[%s] %s %s (no-op)", r.RemoteAddr, r.Method, r.URL.RequestURI()))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"code": 0})
		fmt.Printf("[%s] %s %s [%dms]\n", r.RemoteAddr, r.Method, r.URL.RequestURI(), time.Since(start).Milliseconds())
		return
	}

	logStr := fmt.Sprintf("[%s] %s %s", r.RemoteAddr, r.Method, r.URL.RequestURI())
	if len(bodyBytes) > 0 {
		logStr += " body:" + string(bodyBytes)
	}

	handler := s.route(r.Method, r.URL.Path)
	if handler != nil {
		handler(w, r, bodyBytes)
		elapsed := time.Since(start).Milliseconds()
		writeDataLog(fmt.Sprintf("%s [%dms]", logStr, elapsed))
		fmt.Printf("[%s] %s %s [%dms]\n", r.RemoteAddr, r.Method, r.URL.RequestURI(), elapsed)
	} else {
		writeDataLog(fmt.Sprintf("%s 404", logStr))
		fmt.Printf("[%s] %s %s 404\n", r.RemoteAddr, r.Method, r.URL.RequestURI())
		http.NotFound(w, r)
	}
}

func (s *DispatchServer) route(method, urlPath string) func(http.ResponseWriter, *http.Request, []byte) {
	adminRoute := getConfig().Admin.Route

	switch {
	case method == "GET" && urlPath == adminRoute:
		return s.handleAdminHTML
	case method == "GET" && urlPath == adminRoute+"/api/me":
		return s.handleAdminMe
	case method == "GET" && urlPath == adminRoute+"/api/stats":
		return s.handleAdminStats
	case method == "GET" && urlPath == adminRoute+"/api/users":
		return s.handleAdminUsers
	case method == "POST" && urlPath == adminRoute+"/api/login":
		return s.handleAdminLogin
	case method == "POST" && urlPath == adminRoute+"/api/logout":
		return s.handleAdminLogout
	case method == "POST" && urlPath == adminRoute+"/api/settings":
		return s.handleAdminSettings
	case method == "POST" && urlPath == adminRoute+"/api/users":
		return s.handleAdminCreateUser
	case strings.HasPrefix(urlPath, adminRoute+"/api/users/") && strings.HasSuffix(urlPath, "/uid"):
		return s.handleAdminUpdateUID
	case strings.HasPrefix(urlPath, adminRoute+"/api/users/") && strings.HasSuffix(urlPath, "/password"):
		return s.handleAdminUpdatePassword
	case strings.HasPrefix(urlPath, adminRoute+"/api/users/") && strings.HasSuffix(urlPath, "/ban"):
		return s.handleAdminBan
	case strings.HasPrefix(urlPath, adminRoute+"/api/users/") && strings.HasSuffix(urlPath, "/unban"):
		return s.handleAdminUnban
	case strings.HasPrefix(urlPath, adminRoute+"/api/users/") && strings.HasSuffix(urlPath, "/rename"):
		return s.handleAdminRename
	case strings.HasPrefix(urlPath, adminRoute+"/api/users/") && strings.HasSuffix(urlPath, "/reset-token"):
		return s.handleAdminResetToken
	case method == "POST" && strings.HasPrefix(urlPath, adminRoute+"/api/users/"):
		return s.handleAdminDeleteUser

	case method == "GET" && urlPath == "/query_region_list":
		return s.handleRegionList
	case urlPath == "/query_cur_region" || strings.HasPrefix(urlPath, "/query_cur_region/"):
		return s.handleCurRegion
	case method == "GET" && urlPath == "/query_server_address":
		return s.handleQueryServerAddress
	case method == "GET" && urlPath == "/status/server":
		return s.handleStatusServer

	case method == "GET" && urlPath == "/combo/box/api/config/sdk/combo":
		return s.handleComboConfig
	case method == "GET" && urlPath == "/hk4e_global/combo/granter/api/getConfig":
		return s.handleGranterGetConfig
	case method == "GET" && urlPath == "/hk4e_global/mdk/shield/api/loadConfig":
		return s.handleLoadConfig
	case method == "GET" && urlPath == "/hk4e_global/mdk/agreement/api/getAgreementInfos":
		return s.handleAgreement
	case method == "GET" && urlPath == "/authentication/type":
		return s.handleAuthType
	case strings.HasPrefix(urlPath, "/admin/mi18n/"):
		return s.handleMi18n

	case method == "POST" && (urlPath == "/hk4e_global/mdk/shield/api/login" || urlPath == "/hk4e_cn/mdk/shield/api/login"):
		return s.handleLogin
	case method == "POST" && (urlPath == "/hk4e_global/mdk/shield/api/verify" || urlPath == "/hk4e_cn/mdk/shield/api/verify"):
		return s.handleVerify
	case method == "POST" && (urlPath == "/hk4e_global/combo/granter/login/v2/login" || urlPath == "/hk4e_cn/combo/granter/login/v2/login"):
		return s.handleGranterLogin
	case (method == "POST" || method == "GET") && (urlPath == "/hk4e_global/combo/granter/api/compareProtocolVersion" || urlPath == "/hk4e_cn/combo/granter/api/compareProtocolVersion"):
		return s.handleCompareProtocol
	case method == "POST" && urlPath == "/account/risky/api/check":
		return s.handleRiskyCheck
	case (method == "GET" || method == "POST") && urlPath == "/device-fp/api/getExtList":
		return s.handleDeviceFP
	case method == "POST" && urlPath == "/common/h5log/log/batch":
		return s.handleH5Log
	case method == "POST" && urlPath == "/apm/dataUpload":
		return s.handleAPM
	case method == "POST" && urlPath == "/crash/dataUpload":
		return s.handleAPM
	case method == "POST" && urlPath == "/data_abtest_api/config/experiment/list":
		return s.handleABTest
	case method == "POST" && urlPath == "/api/lang":
		return s.handleSetLang
	}
	return nil
}

func sendJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func sendText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(text))
}

func (s *DispatchServer) handleAdminHTML(w http.ResponseWriter, r *http.Request, body []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(buildAdminHTML()))
}

func getAdminToken(r *http.Request) string {
	if t := r.Header.Get("X-Admin-Token"); t != "" {
		return t
	}
	if t := r.Header.Get("Authorization"); t != "" {
		return t
	}
	cookie := r.Header.Get("Cookie")
	for _, part := range strings.Split(cookie, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "admin_token=") {
			return strings.TrimPrefix(part, "admin_token=")
		}
	}
	return ""
}

func (s *DispatchServer) requireAdmin(w http.ResponseWriter, r *http.Request) string {
	token := getAdminToken(r)
	username := validateAdminSession(token)
	if username == "" {
		sendJSON(w, map[string]interface{}{"error": L("err_not_logged_in")})
	}
	return username
}

func (s *DispatchServer) handleAdminMe(w http.ResponseWriter, r *http.Request, body []byte) {
	username := s.requireAdmin(w, r)
	if username == "" {
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true, "username": username})
}

func (s *DispatchServer) handleAdminStats(w http.ResponseWriter, r *http.Request, body []byte) {
	username := s.requireAdmin(w, r)
	if username == "" {
		return
	}
	stats := getDashboardStats()
	versionStats := getVersionStats()
	hotfixDirs := GetHotfixPlatforms()
	stats["total_requests"] = getHotUpdateCount()
	sendJSON(w, map[string]interface{}{"ok": true, "data": stats, "version_stats": versionStats, "hotfix_platforms": hotfixDirs})
}

func (s *DispatchServer) handleAdminUsers(w http.ResponseWriter, r *http.Request, body []byte) {
	username := s.requireAdmin(w, r)
	if username == "" {
		return
	}
	accounts, err := getAllAccounts()
	if err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	type userJSON struct {
		UID          string `json:"uid"`
		Username     string `json:"username"`
		CreatedAt    string `json:"created_at"`
		LastLogin    string `json:"last_login"`
		Banned       int    `json:"banned"`
		BanReason    string `json:"ban_reason"`
		BanExpiresAt string `json:"ban_expires_at"`
		BanPermanent int    `json:"ban_permanent"`
	}
	var users []userJSON
	for _, a := range accounts {
		users = append(users, userJSON{
			UID: a.UID, Username: a.Username, CreatedAt: a.CreatedAt,
			LastLogin: a.LastLogin, Banned: a.Banned, BanReason: a.BanReason,
			BanExpiresAt: a.BanExpiresAt, BanPermanent: a.BanPermanent,
		})
	}
	sendJSON(w, map[string]interface{}{"ok": true, "data": users})
}

func updateAdminRoute(newRoute string) error {
	return updateConfig(func(c *Config) {
		c.Admin.Route = newRoute
	})
}

func (s *DispatchServer) handleAdminLogin(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	creds, err := getAdminCredentials()
	if err != nil || creds == nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_wrong_creds")})
		return
	}
	if req.Username != creds.Username || !checkPassword(req.Password, creds.PasswordHash) {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_wrong_creds")})
		return
	}
	token := createAdminSession(req.Username)
	sendJSON(w, map[string]interface{}{"ok": true, "token": token, "username": req.Username})
}

func (s *DispatchServer) handleAdminLogout(w http.ResponseWriter, r *http.Request, body []byte) {
	token := 	getAdminToken(r)
	revokeAdminSession(token)
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleAdminSettings(w http.ResponseWriter, r *http.Request, body []byte) {
	token := getAdminToken(r)
	user := validateAdminSession(token)
	if user == "" {
		sendJSON(w, map[string]interface{}{"error": L("err_not_logged_in")})
		return
	}
	var req struct {
		OldPassword string `json:"old_password"`
		NewUsername string `json:"new_username"`
		NewPassword string `json:"new_password"`
		NewRoute    string `json:"new_route"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	creds, err := getAdminCredentials()
	if err != nil || creds == nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_wrong_creds")})
		return
	}
	if !checkPassword(req.OldPassword, creds.PasswordHash) {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_wrong_creds")})
		return
	}
	username := creds.Username
	passwordHash := creds.PasswordHash

	if req.NewUsername != "" {
		if len(req.NewUsername) < 4 || len(req.NewUsername) > 32 {
			sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_username_len")})
			return
		}
		username = req.NewUsername
	}
	if req.NewPassword != "" {
		if len(req.NewPassword) < 4 || len(req.NewPassword) > 32 {
			sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_password_len")})
			return
		}
		h, err := hashPassword(req.NewPassword)
		if err != nil {
			sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		passwordHash = h
	}
	if err := updateAdminCredentials(username, passwordHash); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	if req.NewRoute != "" {
		if err := updateAdminRoute(req.NewRoute); err != nil {
			sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
	}
	revokeAdminSession(token)
	sendJSON(w, map[string]interface{}{"ok": true, "username": username, "logout": true})
}

func (s *DispatchServer) handleAdminCreateUser(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	username := req.Username
	password := req.Password
	if password == "" {
		password = generateToken(8)
	}
	if len(username) < 4 || len(username) > 32 {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_username_len")})
		return
	}
	if len(password) < 4 || len(password) > 32 {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_password_len")})
		return
	}
	existing, err := getAccountByUsername(username)
	if err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	if existing != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_user_exists")})
		return
	}
	_, err = createAccount(username, password)
	if err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func getUsernameFromPath(p string, adminRoute string) string {
	// /admin/api/users/{username}/action
	base := strings.TrimPrefix(p, adminRoute+"/api/users/")
	parts := strings.Split(base, "/")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (s *DispatchServer) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	if username == "" {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_unknown_op")})
		return
	}
	var req struct{ Action string `json:"action"` }
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	if req.Action == "delete" {
		if err := deleteAccount(username); err != nil {
			sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
			return
		}
		sendJSON(w, map[string]interface{}{"ok": true})
	} else {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_unknown_op")})
	}
}

func (s *DispatchServer) handleAdminUpdateUID(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	var req struct{ UID string `json:"uid"` }
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	newUID := strings.TrimSpace(req.UID)
	if newUID == "" {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_uid_empty")})
		return
	}
	if err := updateUID(username, newUID); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleAdminUpdatePassword(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	var req struct{ Password string `json:"password"` }
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	if len(req.Password) < 4 || len(req.Password) > 32 {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_password_len")})
		return
	}
	if err := updateUserPassword(username, req.Password); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleAdminBan(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	var req struct {
		Reason    string `json:"reason"`
		Permanent bool   `json:"permanent"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	var err error
	if req.Permanent {
		err = updateBan(username, req.Reason, true, "")
	} else {
		err = updateBan(username, req.Reason, false, req.ExpiresAt)
	}
	if err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleAdminUnban(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	if err := unbanUser(username); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleAdminRename(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	var req struct{ NewUsername string `json:"new_username"` }
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_invalid_json")})
		return
	}
	newName := strings.TrimSpace(req.NewUsername)
	if newName == "" || len(newName) < 4 || len(newName) > 32 {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_username_len2")})
		return
	}
	existing, err := getAccountByUsername(newName)
	if err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	if existing != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_user_exists")})
		return
	}
	err = renameUser(username, newName)
	if err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": L("err_rename_failed")})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleAdminResetToken(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	username := getUsernameFromPath(r.URL.Path, getConfig().Admin.Route)
	if err := resetUserToken(username); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	sendJSON(w, map[string]interface{}{"ok": true})
}

func (s *DispatchServer) handleRegionList(w http.ResponseWriter, r *http.Request, body []byte) {
	versionCode := r.URL.Query().Get("version")
	sdkenv := "2"
	if strings.HasPrefix(versionCode, "CNRELiOS") || strings.HasPrefix(versionCode, "CNRELWin") || strings.HasPrefix(versionCode, "CNRELAnd") {
		sdkenv = "0"
	}
	rsp := buildRegionList(sdkenv)
	sendText(w, rsp)
}

func getRegionConfig(regionName string) *RegionConfig {
	for _, r := range getConfig().Regions {
		if r.Name == regionName {
			return &r
		}
	}
	return nil
}

func buildRegionList(sdkenv string) string {
	addr := getConfig().Server.AccessAddress
	port := getConfig().Server.AccessPort
	schema := "http"
	if getConfig().Server.TLS.Enable || getConfig().Server.ForcePublicHTTPS {
		schema = "https"
	}
	dispatchDomain := fmt.Sprintf("%s://%s:%d", schema, addr, port)

	var regions [][]byte
	for _, r := range getConfig().Regions {
		rname := r.Name
		rtitle := r.Title
		if rname == "" {
			continue
		}
		dispatchURL := fmt.Sprintf("%s/query_cur_region/%s", dispatchDomain, rname)
		regions = append(regions, BuildRegionSimpleInfo(rname, rtitle, getConfig().RegionType, dispatchURL))
	}

	xorConfig := buildXORConfig(sdkenv)
	rspBytes := BuildQueryRegionListRsp(QueryRegionListRsp{
		Retcode:                     0,
		RegionList:                  regions,
		ClientSecretKey:             dispatchSeed,
		ClientCustomConfigEncrypted: xorConfig,
		EnableLoginPC:               true,
	})
	return base64.StdEncoding.EncodeToString(rspBytes)
}

func buildXORConfig(sdkenv string) []byte {
	configJSON := fmt.Sprintf(`{"sdkenv":"%s","checkdevice":"false","loadPatch":"false","showexception":"false","regionConfig":"pm|fk|add","downloadMode":"0","codeSwitch":[4334],"coverSwitch":[40]}`, sdkenv)
	return xorEncrypt([]byte(configJSON), dispatchKey)
}

func buildRegionInfoFromHotfix(hotfix *HotfixData, ip string, port int, versionStr string) []byte {
	params := hotfix.BuildRegionInfoParams(ip, port, versionStr)
	return BuildRegionInfo(params)
}

func buildDefaultRegionInfo(ip string, port int) []byte {
	return BuildRegionInfo(RegionInfoParams{
		GateserverIP:   ip,
		GateserverPort: uint32(port),
		AreaType:       "CN",
	})
}

func (s *DispatchServer) handleCurRegion(w http.ResponseWriter, r *http.Request, body []byte) {
	incrementHotUpdateCount()

	writeDataLog(fmt.Sprintf("[DISPATCH] %s %s", r.RemoteAddr, r.URL.RequestURI()))

	basePath := strings.TrimPrefix(r.URL.Path, "/query_cur_region")
	basePath = strings.TrimPrefix(basePath, "/")
	parts := strings.Split(basePath, "/")
	regionName := parts[0]
	if len(parts) == 1 && parts[0] == "" {
		parts = parts[:0]
		regionName = ""
	}

	dispatchSeedParam := r.URL.Query().Get("dispatchSeed")
	keyID := r.URL.Query().Get("key_id")
	if keyID == "" {
		keyID = "4"
	}
	version := r.URL.Query().Get("version")

	platform := ""
	if len(parts) > 1 {
		platform = parts[1]
	}
	recordVersionRequest(version, platform)

	stopCfg := getConfig().StopServer
	now := uint32(time.Now().Unix())
	inMaintenance := now >= stopCfg.BeginTime && now < stopCfg.EndTime

	versionNum := extractVersionNum(version)

	regionCfg := getRegionConfig(regionName)
	if regionCfg == nil {
		regionCfg = &RegionConfig{Name: regionName, Title: regionName, Ip: getConfig().GameServer.AccessAddress, Port: getConfig().GameServer.AccessPort}
	}

	ip := regionCfg.Ip
	port := regionCfg.Port
	if ip == "" {
		ip = getConfig().GameServer.AccessAddress
	}
	if port == 0 {
		port = getConfig().GameServer.AccessPort
	}

	if inMaintenance {
		rsp := QueryCurRegionRsp{
			Retcode:    11,
			Msg:        stopCfg.Message,
			StopServer: BuildStopServerInfo(now, now+86400, stopCfg.URL, stopCfg.Message),
		}
		sendCurRegionResponse(w, BuildQueryCurRegionRsp(rsp), keyID, dispatchSeedParam)
		return
	}

	if version == "" || versionNum == "" {
		riBytes := buildDefaultRegionInfo(ip, port)
		rsp := QueryCurRegionRsp{
			Retcode:                    0,
			Msg:                        "OK",
			RegionInfo:                 riBytes,
			ClientSecretKey:            dispatchSeed,
			RegionCustomConfigEncrypted: buildXORConfig("0"),
		}
		sendCurRegionResponse(w, BuildQueryCurRegionRsp(rsp), keyID, dispatchSeedParam)
		return
	}

	hotfix := LoadHotfixConfig(version)
	if hotfix != nil {
		riBytes := buildRegionInfoFromHotfix(hotfix, ip, port, version)
		rsp := QueryCurRegionRsp{
			Retcode:                    0,
			Msg:                        "OK",
			RegionInfo:                 riBytes,
			ClientSecretKey:            dispatchSeed,
			RegionCustomConfigEncrypted: buildXORConfig("0"),
			ConnectGateTicket:          "hotfix",
		}
		sendCurRegionResponse(w, BuildQueryCurRegionRsp(rsp), keyID, dispatchSeedParam)
		return
	}

	// No hotfix config: unsupported version
	rsp := QueryCurRegionRsp{
		Retcode:     20,
		Msg:         getConfig().UnsupportedVersion.Message,
		ForceUpdate: BuildForceUpdateInfo(getConfig().UnsupportedVersion.URL),
	}
	sendCurRegionResponse(w, BuildQueryCurRegionRsp(rsp), keyID, dispatchSeedParam)
}

func sendCurRegionResponse(w http.ResponseWriter, curRegion []byte, keyID string, dispatchSeed string) {
	if dispatchSeed != "" {
		result := encryptAndSignRegionData(curRegion, keyID)
		if result["content"] != "" {
			sendJSON(w, result)
			return
		}
	}
	// Return raw base64 (like hotfix.nyakya.com)
	sendText(w, base64.StdEncoding.EncodeToString(curRegion))
}

func (s *DispatchServer) handleQueryServerAddress(w http.ResponseWriter, r *http.Request, body []byte) {
	addr := fmt.Sprintf("%s:%d", getConfig().GameServer.AccessAddress, getConfig().GameServer.AccessPort)
	sendText(w, addr)
}

func (s *DispatchServer) handleStatusServer(w http.ResponseWriter, r *http.Request, body []byte) {
	count := getHotUpdateCount()
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"status": map[string]interface{}{
			"maxPlayer":   -1,
			"playerCount": count,
			"version":     getConfig().Version,
		},
	})
}

func (s *DispatchServer) handleComboConfig(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data": map[string]interface{}{
			"vals": map[string]string{
				"disable_email_bind_skip":    "false",
				"email_bind_remind_interval": "7",
				"email_bind_remind":          "true",
			},
		},
	})
}

func (s *DispatchServer) handleGranterGetConfig(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data": map[string]interface{}{
			"protocol":                true,
			"qr_enabled":              false,
			"log_level":               "INFO",
			"announce_url":            "https://webstatic-sea.hoyoverse.com/hk4e/announcement/index.html?sdk_presentation_style=fullscreen&sdk_screen_transparent=true&game_biz=hk4e_global&auth_appid=announcement&game=hk4e#/",
			"push_alias_type":         2,
			"disable_ysdk_guard":      false,
			"enable_announce_pic_popup": true,
		},
	})
}

func (s *DispatchServer) handleLoadConfig(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data": map[string]interface{}{
			"id":               6,
			"game_key":         "hk4e_global",
			"client":           "PC",
			"identity":         "I_IDENTITY",
			"guest":            false,
			"ignore_versions":  "",
			"scene":            "S_NORMAL",
			"name":             getConfig().LoadConfig.Name,
			"disable_regist":   false,
			"enable_email_captcha": false,
			"thirdparty":       []string{"fb", "tw"},
			"disable_mmt":      false,
			"server_guest":     false,
			"thirdparty_ignore": map[string]string{"tw": "", "fb": ""},
			"enable_ps_bind_account": false,
			"thirdparty_login_configs": map[string]interface{}{
				"tw": map[string]interface{}{"token_type": "TK_GAME_TOKEN", "game_token_expires_in": 604800},
				"fb": map[string]interface{}{"token_type": "TK_GAME_TOKEN", "game_token_expires_in": 604800},
			},
		},
	})
}

func (s *DispatchServer) handleAgreement(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data":    map[string]interface{}{"marketing_agreements": []interface{}{}},
	})
}

func (s *DispatchServer) handleAuthType(w http.ResponseWriter, r *http.Request, body []byte) {
	sendText(w, "DefaultAuthentication")
}

func (s *DispatchServer) handleMi18n(w http.ResponseWriter, r *http.Request, body []byte) {
	w.WriteHeader(http.StatusOK)
}

func (s *DispatchServer) handleDeviceFP(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data": map[string]interface{}{
			"code": 200,
			"msg":  "ok",
			"device_fp": map[string]interface{}{
				"device_fp":       "none",
				"device_info":     "",
				"device_token":    "",
				"ext":             nil,
			},
		},
	})
}

func (s *DispatchServer) handleH5Log(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{"retcode": 0, "message": "OK"})
}

func (s *DispatchServer) handleAPM(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]int{"code": 0})
}

func displayEmail(a *Account) string {
	if a.Email != "" {
		return a.Email
	}
	return a.Username + "@" + cfg.EmailDomain
}

// --- Login Flow ---

func (s *DispatchServer) handleLogin(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		Account  string `json:"account"`
		Password string `json:"password"`
		Device   string `json:"device"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"message": L("err_invalid_json"), "retcode": -1, "data": nil})
		return
	}

	username := req.Account
	password := req.Password
	deviceID := req.Device

	if username == "" {
		sendJSON(w, map[string]interface{}{
			"message": L("msg_account_invalid"),
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	account, err := getAccountByUsername(username)
	if err != nil {
		sendJSON(w, map[string]interface{}{
			"message": L("msg_account_query_fail"),
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	if account == nil {
		account, err = createAccount(username, password)
		if err != nil {
			sendJSON(w, map[string]interface{}{
				"message": L("msg_account_create_fail"),
				"retcode": -1,
				"data":    nil,
			})
			return
		}
	}

	banned, reason, permanent, banExpiresAt := isUserBannedInfo(username)
	if banned {
		var msg string
		if permanent {
			msg = L("msg_account_banned_perm")
		} else {
			if reason == "" {
				reason = L("msg_account_banned")
			}
			msg = reason + "\n" + L("msg_ban_unban_at") + banExpiresAt
		}
		sendJSON(w, map[string]interface{}{
			"message": msg,
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	uid := account.UID
	sessionKey := updateSessionKey(uid, deviceID)

	sendJSON(w, map[string]interface{}{
		"message": "OK",
		"retcode": 0,
		"data": map[string]interface{}{
			"account": map[string]interface{}{
				"uid":              uid,
				"name":             username,
				"email":            displayEmail(account),
				"mobile":           "",
				"is_email_verify":  "0",
				"realname":         "",
				"identity_card":    "",
				"token":            sessionKey,
				"safe_mobile":      "",
				"facebook_name":    "",
				"twitter_name":     "",
				"game_center_name": "",
				"google_name":      "",
				"apple_name":       "",
				"sony_name":        "",
				"tap_name":         "",
				"country":          "US",
				"reactivate_ticket":"",
				"area_code":        "**",
				"device_grant_ticket": "",
			},
			"device_grant_required": false,
			"realname_operation":    "NONE",
			"realperson_required":   false,
			"safe_mobile_required":  false,
		},
	})
}

func (s *DispatchServer) handleVerify(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		UID   string `json:"uid"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"message": L("err_invalid_json"), "retcode": -1, "data": nil})
		return
	}

	uid := req.UID
	token := req.Token

	account, err := getAccountByUID(uid)
	if err != nil {
		sendJSON(w, map[string]interface{}{"message": L("msg_token_invalid"), "retcode": -1, "data": nil})
		return
	}
	if account == nil || account.SessionKey != token {
		sendJSON(w, map[string]interface{}{
			"message": L("msg_token_invalid"),
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	if isTokenExpired(uid) {
		sendJSON(w, map[string]interface{}{
			"message": L("msg_token_expired"),
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	banned, reason, permanent, banExpiresAt := isUserBannedInfo(account.Username)
	if banned {
		var msg string
		if permanent {
			msg = L("msg_account_banned_perm")
		} else {
			if reason == "" {
				reason = L("msg_account_banned")
			}
			msg = reason + "\n" + L("msg_ban_unban_at") + banExpiresAt
		}
		sendJSON(w, map[string]interface{}{
			"message": msg,
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	extendTokenExpiry(uid)

	sendJSON(w, map[string]interface{}{
		"message": "OK",
		"retcode": 0,
		"data": map[string]interface{}{
			"account": map[string]interface{}{
				"uid":              uid,
				"name":             account.Username,
				"email":            displayEmail(account),
				"mobile":           "",
				"is_email_verify":  "0",
				"realname":         "",
				"identity_card":    "",
				"token":            token,
				"safe_mobile":      "",
				"facebook_name":    "",
				"twitter_name":     "",
				"game_center_name": "",
				"google_name":      "",
				"apple_name":       "",
				"sony_name":        "",
				"tap_name":         "",
				"country":          "US",
				"reactivate_ticket":"",
				"area_code":        "**",
				"device_grant_ticket": "",
			},
			"device_grant_required": false,
			"realname_operation":    "NONE",
			"realperson_required":   false,
			"safe_mobile_required":  false,
		},
	})
}

func (s *DispatchServer) handleGranterLogin(w http.ResponseWriter, r *http.Request, body []byte) {
	var req struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"message": L("err_invalid_json"), "retcode": -1, "data": nil})
		return
	}

	var tokenData struct {
		UID   string `json:"uid"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(req.Data), &tokenData); err != nil {
		sendJSON(w, map[string]interface{}{"message": L("err_invalid_json"), "retcode": -1, "data": nil})
		return
	}

	uid := tokenData.UID
	token := tokenData.Token

	account, err := getAccountByUID(uid)
	if err != nil {
		sendJSON(w, map[string]interface{}{"message": L("msg_token_invalid"), "retcode": -1, "data": nil})
		return
	}
	if account == nil || account.SessionKey != token {
		sendJSON(w, map[string]interface{}{
			"message": L("msg_session_invalid"),
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	if isTokenExpired(uid) {
		sendJSON(w, map[string]interface{}{
			"message": L("msg_token_expired"),
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	banned, reason, permanent, banExpiresAt := isUserBannedInfo(account.Username)
	if banned {
		var msg string
		if permanent {
			msg = L("msg_account_banned_perm")
		} else {
			if reason == "" {
				reason = L("msg_account_banned")
			}
			msg = reason + "\n" + L("msg_ban_unban_at") + banExpiresAt
		}
		sendJSON(w, map[string]interface{}{
			"message": msg,
			"retcode": -1,
			"data":    nil,
		})
		return
	}

	loginToken := updateAccountLoginToken(account.Username)

	sendJSON(w, map[string]interface{}{
		"message": "OK",
		"retcode": 0,
		"data": map[string]interface{}{
			"account_type":  1,
			"heartbeat":     false,
			"combo_id":      "157795300",
			"combo_token":   loginToken,
			"open_id":       uid,
			"data":          `{"guest":false}`,
			"fatigue_remind": nil,
		},
	})
}

func (s *DispatchServer) handleCompareProtocol(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data": map[string]interface{}{
			"modified": true,
			"protocol": map[string]interface{}{
				"id":             0,
				"app_id":         4,
				"language":       "en",
				"user_proto":     "",
				"priv_proto":     "",
				"major":          7,
				"minimum":        0,
				"create_time":    "0",
				"teenager_proto": "",
				"third_proto":    "",
			},
		},
	})
}

func (s *DispatchServer) handleRiskyCheck(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"message": "OK",
		"data": map[string]interface{}{
			"id":      "none",
			"action":  "ACTION_NONE",
			"geetest": nil,
		},
	})
}

func (s *DispatchServer) handleABTest(w http.ResponseWriter, r *http.Request, body []byte) {
	sendJSON(w, map[string]interface{}{
		"retcode": 0,
		"success": true,
		"message": "",
		"data": []map[string]interface{}{
			{
				"code":      1000,
				"type":      2,
				"config_id": "14",
				"period_id": "6036_99",
				"version":   "1",
				"configs":   map[string]string{"cardType": "old"},
			},
		},
	})
}

func (s *DispatchServer) handleSetLang(w http.ResponseWriter, r *http.Request, body []byte) {
	if s.requireAdmin(w, r) == "" {
		return
	}
	var req struct {
		Lang int `json:"lang"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": "invalid json"})
		return
	}
	if req.Lang != 0 && req.Lang != 1 {
		sendJSON(w, map[string]interface{}{"ok": false, "error": "invalid lang"})
		return
	}
	if err := updateConfig(func(c *Config) {
		c.Language = req.Lang
	}); err != nil {
		sendJSON(w, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	loadLang(configPath)
	sendJSON(w, map[string]interface{}{"ok": true})
}

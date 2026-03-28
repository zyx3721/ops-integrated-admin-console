package runtime

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ops-admin-backend/internal/project"

	"golang.org/x/crypto/bcrypt"
)

func (s *server) route(w http.ResponseWriter, r *http.Request) {
	if (r.URL.Path == "/health" || r.URL.Path == "/api/health") && r.Method == http.MethodGet {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return
	}
	if r.URL.Path == "/api/auth/login" && r.Method == http.MethodPost {
		s.handleLogin(w, r)
		return
	}
	if r.URL.Path == "/api/auth/register" && r.Method == http.MethodPost {
		s.handleRegister(w, r)
		return
	}
	if r.URL.Path == "/api/auth/me" && r.Method == http.MethodGet {
		s.requireAuth(s.handleMe)(w, r)
		return
	}
	if r.URL.Path == "/api/auth/logout" && r.Method == http.MethodPost {
		s.requireAuth(s.handleLogout)(w, r)
		return
	}
	if r.URL.Path == "/api/auth/change-password" && r.Method == http.MethodPost {
		s.requireAuth(s.handleChangePassword)(w, r)
		return
	}
	if r.URL.Path == "/api/projects/credentials" && r.Method == http.MethodGet {
		s.requireAuth(s.handleProjectCredentials)(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/projects/credentials/") && r.Method == http.MethodPut {
		s.requireAuth(s.handleProjectCredentialByType)(w, r)
		return
	}
	if r.URL.Path == "/api/projects/relogin" && r.Method == http.MethodPost {
		s.requireAuth(s.handleProjectsRelogin)(w, r)
		return
	}
	if r.URL.Path == "/api/projects/operate-async" && r.Method == http.MethodPost {
		s.requireAuth(s.handleProjectOperateAsyncStart)(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/projects/operate-async/") && r.Method == http.MethodGet {
		s.requireAuth(s.handleProjectOperateAsyncStatus)(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/projects/") {
		s.requireAuth(s.handleProjectOps)(w, r)
		return
	}
	if r.URL.Path == "/api/logs" && r.Method == http.MethodGet {
		s.requireAuth(s.handleLogs)(w, r)
		return
	}
	writeJSON(w, http.StatusNotFound, apiError{Error: "接口不存在"})
}

func (s *server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "请求体格式错误"})
		return
	}
	if strings.TrimSpace(req.Username) == "" || strings.TrimSpace(req.Password) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "用户名和密码不能为空"})
		return
	}

	var adminCount int
	if err := s.db.QueryRow(`SELECT COUNT(1) FROM admins`).Scan(&adminCount); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "查询管理员失败"})
		return
	}
	if adminCount == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "暂无管理员账号，请先注册"})
		return
	}

	var userID int64
	var username, hash string
	err := s.db.QueryRow(`SELECT id,username,password_hash FROM admins WHERE username=?`, req.Username).Scan(&userID, &username, &hash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusUnauthorized, apiError{Error: "账号或密码错误"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "查询管理员失败"})
		return
	}

	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
		writeJSON(w, http.StatusUnauthorized, apiError{Error: "账号或密码错误"})
		return
	}

	token, err := randomToken(48)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "生成令牌失败"})
		return
	}
	exp := time.Now().Add(s.tokenTTL)
	now := nowStr()
	if _, err = s.db.Exec(`INSERT INTO auth_tokens(token,user_id,expires_at,created_at) VALUES(?,?,?,?)`, token, userID, exp.Format(time.RFC3339), now); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "创建登录会话失败"})
		return
	}
	if err = ensureDefaultProjectCredentialsForUser(s.db, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "初始化项目凭据失败"})
		return
	}

	s.logAction(userID, username, "login", "", "用户登录成功")
	writeJSON(w, http.StatusOK, loginResp{
		Token:                token,
		Username:             username,
		ExpireAt:             exp.Format(time.RFC3339),
		DefaultPwd:           false,
		ProjectCacheTTLInSec: int(s.cfg.ProjectCacheTTL.Seconds()),
		SessionIdleTTLInSec:  int(s.cfg.SessionIdleTTL.Seconds()),
	})
}

func (s *server) handleMe(w http.ResponseWriter, _ *http.Request, u authedUser) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"id":                        u.ID,
		"username":                  u.Username,
		"project_cache_ttl_seconds": int(s.cfg.ProjectCacheTTL.Seconds()),
		"session_idle_ttl_seconds":  int(s.cfg.SessionIdleTTL.Seconds()),
	})
}

func (s *server) handleLogout(w http.ResponseWriter, _ *http.Request, u authedUser) {
	s.cleanupUserAuthTokens(u.ID)
	s.logAction(u.ID, u.Username, "logout", "", "管理员退出登录")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "请求体格式错误"})
		return
	}
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "用户名和密码不能为空"})
		return
	}
	if len(username) < 3 || len(username) > 32 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "用户名长度必须为3-32位"})
		return
	}
	if len(password) < 8 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "密码长度至少8位"})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "密码加密失败"})
		return
	}
	res, err := s.db.Exec(`INSERT INTO admins(username,password_hash,created_at,updated_at) VALUES(?,?,?,?)`, username, string(hash), nowStr(), nowStr())
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			writeJSON(w, http.StatusConflict, apiError{Error: "用户名已存在"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "创建用户失败"})
		return
	}
	userID, _ := res.LastInsertId()
	if err = ensureDefaultProjectCredentialsForUser(s.db, userID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "初始化项目凭据失败"})
		return
	}
	s.logAction(userID, username, "register", "", "管理员注册成功")
	writeJSON(w, http.StatusOK, map[string]string{"message": "注册成功"})
}

func (s *server) handleChangePassword(w http.ResponseWriter, r *http.Request, u authedUser) {
	var req changePasswordReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "请求体格式错误"})
		return
	}
	if strings.TrimSpace(req.OldPassword) == "" || strings.TrimSpace(req.NewPassword) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "原密码和新密码不能为空"})
		return
	}
	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "新密码长度至少8位"})
		return
	}

	var hash string
	if err := s.db.QueryRow(`SELECT password_hash FROM admins WHERE id=?`, u.ID).Scan(&hash); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "查询当前密码失败"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)) != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "原密码错误"})
		return
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "密码加密失败"})
		return
	}
	if _, err = s.db.Exec(`UPDATE admins SET password_hash=?,updated_at=? WHERE id=?`, string(newHash), nowStr(), u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "更新密码失败"})
		return
	}
	s.logAction(u.ID, u.Username, "change_password", "", "管理员修改密码")
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码修改成功"})
}

func (s *server) handleProjectCredentials(w http.ResponseWriter, _ *http.Request, u authedUser) {
	if err := ensureDefaultProjectCredentialsForUser(s.db, u.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "初始化项目凭据失败"})
		return
	}
	rows, err := s.db.Query(`SELECT project_type,account,password,updated_at FROM project_credentials WHERE user_id=? ORDER BY project_type`, u.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "查询项目凭据失败"})
		return
	}
	defer rows.Close()

	items := make([]map[string]string, 0)
	for rows.Next() {
		var t, account, password, updated string
		if err = rows.Scan(&t, &account, &password, &updated); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "读取项目凭据失败"})
			return
		}
		plainPwd, decErr := decryptCredentialPassword(password, s.cfg.CredentialKey)
		if decErr != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "项目凭据解密失败"})
			return
		}
		items = append(items, map[string]string{"project_type": t, "account": account, "password": plainPwd, "updated_at": updated})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": items})
}

func (s *server) handleProjectCredentialByType(w http.ResponseWriter, r *http.Request, u authedUser) {
	projectType := strings.TrimPrefix(r.URL.Path, "/api/projects/credentials/")
	if !validCredentialProjectType(projectType) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "无效的项目类型"})
		return
	}
	var req projectCredentialReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "请求体格式错误"})
		return
	}
	if strings.TrimSpace(req.Account) == "" || strings.TrimSpace(req.Password) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "账号和密码不能为空"})
		return
	}
	encryptedPwd, err := encryptCredentialPassword(req.Password, s.cfg.CredentialKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "凭据加密失败"})
		return
	}
	if _, err = s.db.Exec(`INSERT INTO project_credentials(user_id,project_type,account,password,updated_at) VALUES(?,?,?,?,?)
	ON CONFLICT(user_id,project_type) DO UPDATE SET account=excluded.account,password=excluded.password,updated_at=excluded.updated_at`,
		u.ID, projectType, req.Account, encryptedPwd, nowStr(),
	); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "更新项目凭据失败"})
		return
	}
	s.projectSessions.clearUserProject(u.ID, projectType)
	s.logAction(u.ID, u.Username, "update_project_credential", projectType, "更新项目凭据")
	writeJSON(w, http.StatusOK, map[string]string{"message": "更新成功"})
}

func (s *server) handleProjectOps(w http.ResponseWriter, r *http.Request, u authedUser) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 4 || parts[0] != "api" || parts[1] != "projects" {
		writeJSON(w, http.StatusNotFound, apiError{Error: "接口不存在"})
		return
	}
	projectType, op := parts[2], parts[3]
	if !validProjectType(projectType) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "无效的项目类型"})
		return
	}
	if op == "load" && r.Method == http.MethodPost {
		s.handleProjectLoad(w, u, projectType)
		return
	}
	if op == "batch-template" && r.Method == http.MethodGet {
		s.handleProjectBatchTemplate(w, r, projectType)
		return
	}
	if op == "batch-upload" && r.Method == http.MethodPost {
		s.handleProjectBatchUpload(w, r, projectType)
		return
	}
	if op == "batch-files" && r.Method == http.MethodGet {
		s.handleProjectBatchFiles(w, projectType)
		return
	}
	if op == "operate" && r.Method == http.MethodPost {
		s.handleProjectOperate(w, r, u, projectType)
		return
	}
	writeJSON(w, http.StatusNotFound, apiError{Error: "接口不存在"})
}

func (s *server) handleProjectLoad(w http.ResponseWriter, u authedUser, projectType string) {
	_, didLogin, message, err := s.ensureProjectSession(u, projectType, false)
	if err != nil {
		s.logAction(u.ID, u.Username, "project_load_failed", projectType, truncate(err.Error(), 600))
		writeJSON(w, http.StatusBadGateway, apiError{Error: err.Error()})
		return
	}
	if !didLogin {
		writeJSON(w, http.StatusOK, map[string]interface{}{"loaded": true, "first_load": false, "session_state": "reused"})
		return
	}
	s.logAction(u.ID, u.Username, "project_load", projectType, "首次加载完成")
	writeJSON(w, http.StatusOK, map[string]interface{}{"loaded": true, "first_load": true, "message": message, "session_state": "first_login"})
}

func (s *server) handleProjectBatchFiles(w http.ResponseWriter, projectType string) {
	if projectType != "ad" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "批量文件仅支持AD项目"})
		return
	}
	files, err := project.BatchExcelFiles()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	items := make([]map[string]string, 0, len(files))
	for _, name := range files {
		items = append(items, map[string]string{
			"name": name,
			"path": filepath.Join(project.BatchUploadDir(), name),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items": items,
		"dir":   project.BatchUploadDir(),
	})
}

func (s *server) handleProjectBatchTemplate(w http.ResponseWriter, r *http.Request, projectType string) {
	if projectType != "ad" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "批量模板仅支持AD项目"})
		return
	}
	path := project.BatchTemplatePath()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusNotFound, apiError{Error: "模板文件不存在"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	filename := filepath.Base(path)
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(filename)))
	http.ServeFile(w, r, path)
}

func (s *server) handleProjectBatchUpload(w http.ResponseWriter, r *http.Request, projectType string) {
	if projectType != "ad" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "批量上传仅支持AD项目"})
		return
	}
	if err := os.MkdirAll(project.BatchUploadDir(), 0o755); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "无效的表单数据"})
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "文件不能为空"})
		return
	}
	defer file.Close()

	oldFile := filepath.Base(strings.TrimSpace(r.FormValue("old_file")))

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".xlsx" && ext != ".xls" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "仅支持上传 xlsx/.xls 文件"})
		return
	}

	storedName := fmt.Sprintf("ad_batch_%d%s", time.Now().UnixNano(), ext)
	outPath := filepath.Join(project.BatchUploadDir(), storedName)
	outFile, err := os.Create(outPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: err.Error()})
		return
	}
	defer outFile.Close()
	if _, err = io.Copy(outFile, file); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "保存文件失败"})
		return
	}

	if oldFile != "" {
		_ = os.Remove(filepath.Join(project.BatchUploadDir(), oldFile))
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"name":          storedName,
		"original_name": header.Filename,
		"path":          outPath,
	})
}

func (s *server) handleProjectOperate(w http.ResponseWriter, r *http.Request, u authedUser, projectType string) {
	var req operateReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "请求体格式错误"})
		return
	}
	if strings.TrimSpace(req.Action) == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "操作类型不能为空"})
		return
	}
	if req.Params == nil {
		req.Params = map[string]interface{}{}
	}
	if projectType == "vpn" && req.Action == "delete_users" && toBoolDefault(req.Params["remote_firewall"], false) {
		fwAccount, fwPassword, fwErr := s.getProjectCredential(u.ID, "vpn_firewall")
		if fwErr != nil {
			req.Params["__vpn_fw_configured"] = false
			req.Params["__vpn_fw_error"] = fwErr.Error()
		} else {
			req.Params["__vpn_fw_configured"] = true
			req.Params["__vpn_fw_account"] = fwAccount
			req.Params["__vpn_fw_password"] = fwPassword
		}
	}

	entry, didLogin, _, err := s.ensureProjectSession(u, projectType, false)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	result, err := s.operateWithProjectSession(entry, req.Action, req.Params)
	if err != nil {
		s.logAction(u.ID, u.Username, "project_operate_failed", projectType, fmt.Sprintf("action=%s, err=%v", req.Action, err))
		writeJSON(w, http.StatusBadGateway, apiError{Error: err.Error()})
		return
	}
	if !result.OK {
		errMsg := result.Error
		if errMsg == "" {
			errMsg = result.Message
		}
		s.logAction(u.ID, u.Username, "project_operate_failed", projectType, fmt.Sprintf("action=%s, err=%s", req.Action, errMsg))
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": errMsg, "message": result.Message, "data": result.Data})
		return
	}
	s.logAction(u.ID, u.Username, "project_operate", projectType, fmt.Sprintf("action=%s", req.Action))
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": result.Message, "data": result.Data, "session_state": projectSessionStateFromDidLogin(didLogin)})
}

func (s *server) handleProjectsRelogin(w http.ResponseWriter, _ *http.Request, u authedUser) {
	s.projectSessions.clearToken(u.Token)
	reloginItems := make([]map[string]interface{}, 0, 3)
	for _, projectType := range []string{"ad", "print", "vpn"} {
		_, _, message, err := s.ensureProjectSession(u, projectType, true)
		if err != nil {
			reloginItems = append(reloginItems, map[string]interface{}{
				"project_type": projectType,
				"ok":           false,
				"message":      err.Error(),
			})
			continue
		}
		reloginItems = append(reloginItems, map[string]interface{}{
			"project_type":  projectType,
			"ok":            true,
			"message":       message,
			"session_state": "countdown_relogin",
		})
	}
	s.logAction(u.ID, u.Username, "project_relogin", "", "手动触发项目重新登录")
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":           reloginItems,
		"next_cleanup_at": time.Now().Add(s.cfg.ProjectCacheTTL).Format(time.RFC3339),
	})
}

func (s *server) handleLogs(w http.ResponseWriter, r *http.Request, _ authedUser) {
	page := 1
	pageSize := 20

	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 && n <= 1000 {
			pageSize = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("page")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			page = n
		}
	}
	if v := strings.TrimSpace(r.URL.Query().Get("page_size")); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 200 {
		pageSize = 200
	}

	projectType := strings.TrimSpace(r.URL.Query().Get("project_type"))
	where := ""
	countArgs := make([]interface{}, 0, 1)
	if projectType != "" {
		where = ` WHERE project_type=?`
		countArgs = append(countArgs, projectType)
	}

	var total int
	countQuery := `SELECT COUNT(1) FROM operation_logs` + where
	if err := s.db.QueryRow(countQuery, countArgs...).Scan(&total); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "查询日志失败"})
		return
	}

	offset := (page - 1) * pageSize
	query := `SELECT id,COALESCE(user_id,0),COALESCE(username,''),COALESCE(action,''),COALESCE(project_type,''),COALESCE(detail,''),created_at FROM operation_logs` +
		where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	args := make([]interface{}, 0, len(countArgs)+2)
	args = append(args, countArgs...)
	args = append(args, pageSize, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "查询日志失败"})
		return
	}
	defer rows.Close()
	items := make([]logRow, 0)
	for rows.Next() {
		var row logRow
		if err = rows.Scan(&row.ID, &row.UserID, &row.Username, &row.Action, &row.ProjectType, &row.Detail, &row.CreatedAt); err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "读取日志失败"})
			return
		}
		row.Detail = normalizeGarbledText(row.Detail)
		items = append(items, row)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"items":     items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (s *server) getProjectCredential(userID int64, projectType string) (string, string, error) {
	var account, password string
	err := s.db.QueryRow(`SELECT account,password FROM project_credentials WHERE user_id=? AND project_type=?`, userID, projectType).Scan(&account, &password)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", "", errors.New("项目凭据未配置")
		}
		return "", "", err
	}
	password, err = decryptCredentialPassword(password, s.cfg.CredentialKey)
	if err != nil {
		return "", "", errors.New("凭据解密失败")
	}
	if strings.TrimSpace(account) == "" || strings.TrimSpace(password) == "" {
		return "", "", errors.New("项目凭据未配置")
	}
	return account, password, nil
}

func (s *server) requireAuth(next func(http.ResponseWriter, *http.Request, authedUser)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, apiError{Error: "缺少认证令牌"})
			return
		}
		now := time.Now().Format(time.RFC3339)
		u, err := s.loadAuthedUser(token, now)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, apiError{Error: "令牌无效或已过期"})
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, u)), u)
	}
}

func (s *server) logAction(userID int64, username, action, projectType, detail string) {
	detail = normalizeGarbledText(detail)
	_, _ = s.db.Exec(`INSERT INTO operation_logs(user_id,username,action,project_type,detail,created_at) VALUES(?,?,?,?,?,?)`, userID, username, action, projectType, detail, nowStr())
}

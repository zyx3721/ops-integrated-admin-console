package project

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

func adLogin(username, password string) (*http.Client, error) {
	client := newHTTPClient(25 * time.Second)
	form := url.Values{}
	form.Set("Username", username)
	form.Set("Password", password)
	resp, err := postForm(client, adEndpoint("userlogin/"), form)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ad login http=%d body=%s", resp.StatusCode, truncate(string(body), 120))
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return nil, err
	}
	if toInt(data["code"]) != 4 {
		return nil, fmt.Errorf("ad login failed: %v", data)
	}
	return client, nil
}

func adSearchRaw(client *http.Client, search string) (map[string]interface{}, error) {
	payload := url.Values{}
	payload.Set("searchvalue", search)
	payload.Set("NameList", "用户")
	resp, err := postForm(client, adEndpoint("api/GetLeaveUser/"), payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ad search http=%d body=%s", resp.StatusCode, truncate(string(body), 120))
	}
	return decodeRespJSON(resp)
}

func adFindDN(client *http.Client, username string) (string, error) {
	data, err := adSearchRaw(client, username)
	if err != nil {
		return "", err
	}
	for _, one := range toSlice(data["message"]) {
		m, ok := one.(map[string]interface{})
		if !ok {
			continue
		}
		if toString(m["sAMAccountName"]) == username {
			return toString(m["distinguishedName"]), nil
		}
	}
	return "", nil
}

func adOperate(client *http.Client, action string, p map[string]interface{}) projectResult {
	switch action {
	case "add_user":
		return adAddUser(client, p)
	case "batch_add_users":
		return adBatchAddUsers(client, p)
	case "search_user":
		return adSearchUsers(client, p)
	case "reset_password":
		return adResetPassword(client, p)
	case "unlock_user":
		return adUnlockUser(client, p)
	case "modify_description":
		return adModifyDescription(client, p)
	case "modify_name":
		return adModifyName(client, p)
	case "delete_user":
		return adDeleteUser(client, p)
	default:
		return projectResult{OK: false, Message: "不支持的AD操作", Error: "不支持的操作"}
	}
}
func adAddUser(client *http.Client, p map[string]interface{}) projectResult {
	password := strings.TrimSpace(toString(p["password"]))
	if password == "" {
		password = randomPassword()
	}
	if !isValidStrongPassword(password) {
		return projectResult{OK: false, Message: "新增用户失败", Error: "密码至少8位，且包含大小写字母和数字"}
	}
	username := strings.TrimSpace(toString(p["username"]))
	if username == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "用户名不能为空"}
	}
	if strings.TrimSpace(toString(p["cn"])) == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "姓名不能为空"}
	}
	email := strings.TrimSpace(toString(p["email"]))
	if email == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "邮箱不能为空"}
	}
	if !isValidEmail(email) {
		return projectResult{OK: false, Message: "新增用户失败", Error: "邮箱格式不正确"}
	}

	payload := url.Values{}
	payload.Set("add_user_distinguishedName", fmt.Sprintf("OU=Users,OU=%s,DC=vdesktop,DC=sunline,DC=cn", toString(p["ou"])))
	payload.Set("add_user_sn", toString(p["sn"]))
	payload.Set("add_user_givenName", toString(p["given_name"]))
	payload.Set("add_user_cn", toString(p["cn"]))
	payload.Set("add_user_userPrincipalName2", "@vdesktop.sunline.cn")
	payload.Set("add_user_sAMAccountName1", "vdesktop\\")
	payload.Set("add_user_sAMAccountName2", username)
	payload.Set("add_user_password", password)
	payload.Set("add_user_mail2", email)
	payload.Set("add_user_description", toString(p["description"]))
	payload.Set("add_user_userAccountControl", "yes")

	resp, err := postForm(client, adEndpoint("addUser/"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "新增用户失败", Error: "执行失败: " + err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		detail := strings.TrimSpace(string(body))
		if detail == "" {
			detail = http.StatusText(resp.StatusCode)
		}
		return projectResult{
			OK:      false,
			Message: "新增用户失败",
			Error:   fmt.Sprintf("执行失败！状态码: %d, 响应: %s", resp.StatusCode, truncate(detail, 200)),
		}
	}

	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "新增用户失败", Error: "响应解析失败: " + err.Error()}
	}
	if toBool(data["isSuccess"]) {
		logText := fmt.Sprintf("用户名：%s\n密码：%s", username, password)
		return projectResult{OK: true, Message: "新增用户成功", Data: map[string]interface{}{"username": username, "password": password, "raw": data, "log_text": logText}}
	}

	msg := strings.TrimSpace(toString(data["message"]))
	if msg == "" {
		msg = strings.TrimSpace(toString(data["msg"]))
	}
	if msg == "" || strings.Contains(strings.ToLower(msg), "exist") {
		msg = fmt.Sprintf("添加失败，AD用户 %s 已存在！", username)
	} else if !strings.HasPrefix(msg, "添加失败") && !strings.HasPrefix(msg, "新增用户失败") {
		msg = "新增用户失败: " + msg
	}

	return projectResult{OK: false, Message: "新增用户失败", Error: msg, Data: map[string]interface{}{"raw": data}}
}

func adBatchAddUsers(client *http.Client, p map[string]interface{}) projectResult {
	rows := toSlice(p["rows"])
	records := make([]map[string]interface{}, 0, len(rows))
	for _, one := range rows {
		m, ok := one.(map[string]interface{})
		if !ok {
			continue
		}
		records = append(records, m)
	}
	if len(records) == 0 {
		excelFile := strings.TrimSpace(toString(p["excel_file"]))
		if excelFile == "" {
			files, err := adBatchExcelFiles()
			if err != nil {
				return projectResult{OK: false, Message: "读取Excel文件列表失败", Error: err.Error()}
			}
			if len(files) == 1 {
				excelFile = files[0]
			} else {
				return projectResult{OK: false, Message: "请先选择Excel文件", Error: "请先选择Excel文件"}
			}
		}
		excelPath, err := adResolveBatchExcelPath(excelFile)
		if err != nil {
			return projectResult{OK: false, Message: "Excel文件无效", Error: err.Error()}
		}
		records, err = adReadBatchRowsFromExcel(excelPath)
		if err != nil {
			return projectResult{OK: false, Message: "读取Excel失败", Error: err.Error()}
		}
	}
	if len(records) == 0 {
		return projectResult{OK: false, Message: "Excel没有可用数据", Error: "Excel没有可用数据"}
	}

	okCount := 0
	items := make([]map[string]interface{}, 0, len(records))
	for idx, m := range records {
		res := adAddUser(client, m)
		user := toString(m["username"])
		pwd := toString(m["password"])
		if data := res.Data; data != nil {
			if u := toString(data["username"]); strings.TrimSpace(u) != "" {
				user = u
			}
			if p := toString(data["password"]); strings.TrimSpace(p) != "" {
				pwd = p
			}
		}
		errorReason := ""
		if !res.OK {
			errorReason = strings.TrimSpace(res.Error)
			if errorReason == "" {
				errorReason = strings.TrimSpace(res.Message)
			}
			pwd = ""
		}

		item := map[string]interface{}{
			"ok":           res.OK,
			"username":     user,
			"password":     pwd,
			"error_reason": errorReason,
			"message":      res.Message,
			"error":        res.Error,
		}
		if res.OK {
			okCount++
			item["message"] = fmt.Sprintf("AD用户：%s 密码：%s", user, pwd)
			emitProgress(p, fmt.Sprintf("用户 %s 新增成功", user), idx+1, len(records))
		} else {
			emitProgress(p, fmt.Sprintf("用户 %s 新增失败：%s", user, errorReason), idx+1, len(records))
		}
		items = append(items, item)
	}
	return projectResult{OK: true, Message: fmt.Sprintf("批量新增完成，成功 %d/%d", okCount, len(records)), Data: map[string]interface{}{"items": items}}
}

func adBatchUploadDir() string {
	return filepath.Clean("./data/ad/uploads")
}

func adBatchTemplatePath() string {
	return filepath.Clean("./data/ad/templates/创建AD用户模板.xlsx")
}

func adBatchExcelFiles() ([]string, error) {
	dir := adBatchUploadDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("prepare ad upload dir failed: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read ad upload dir failed: %w", err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext == ".xlsx" || ext == ".xls" {
			files = append(files, entry.Name())
		}
	}
	sort.Strings(files)
	return files, nil
}

func adResolveBatchExcelPath(excelFile string) (string, error) {
	name := filepath.Base(strings.TrimSpace(excelFile))
	if name == "" {
		return "", errors.New("excel_file required")
	}
	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".xlsx" && ext != ".xls" {
		return "", errors.New("excel file must be .xlsx or .xls")
	}
	path := filepath.Join(adBatchUploadDir(), name)
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("excel file not found: %s", name)
		}
		return "", fmt.Errorf("stat excel file failed: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("excel path is directory: %s", name)
	}
	return path, nil
}

func adReadBatchRowsFromExcel(excelPath string) ([]map[string]interface{}, error) {
	f, err := excelize.OpenFile(excelPath)
	if err != nil {
		return nil, fmt.Errorf("open excel failed: %w", err)
	}
	defer f.Close()
	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("no sheet found")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read excel rows failed: %w", err)
	}
	if len(rows) <= 1 {
		return nil, errors.New("excel has no data rows")
	}
	records := make([]map[string]interface{}, 0, len(rows)-1)
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		sn := strings.TrimSpace(cellAt(row, 0))
		givenName := strings.TrimSpace(cellAt(row, 1))
		cn := strings.TrimSpace(cellAt(row, 2))
		username := strings.TrimSpace(cellAt(row, 3))
		email := strings.TrimSpace(cellAt(row, 4))
		description := strings.TrimSpace(cellAt(row, 5))
		ou := strings.TrimSpace(cellAt(row, 6))

		if sn == "" && givenName == "" && cn == "" && username == "" && email == "" && description == "" && ou == "" {
			continue
		}
		if cn == "" {
			cn = sn + givenName
		}
		if !isValidEmail(email) {
			return nil, fmt.Errorf("invalid email in row %d: %s", i+1, email)
		}
		if cn == "" || username == "" || ou == "" {
			return nil, fmt.Errorf("missing required values in row %d", i+1)
		}

		records = append(records, map[string]interface{}{
			"sn":          sn,
			"given_name":  givenName,
			"cn":          cn,
			"username":    username,
			"password":    randomPassword(),
			"email":       email,
			"description": description,
			"ou":          ou,
		})
	}
	return records, nil
}

func adRoleTextFromMessage(m map[string]interface{}) string {
	if m == nil {
		return ""
	}
	for _, key := range []string{"memberOf", "memberof", "roles", "role", "groups", "group"} {
		if text := adNormalizeRoleValue(m[key]); text != "" {
			return text
		}
	}
	return ""
}

func adNormalizeRoleValue(v interface{}) string {
	if v == nil {
		return ""
	}
	values := make([]string, 0, 4)
	switch vv := v.(type) {
	case string:
		if s := strings.TrimSpace(vv); s != "" {
			values = append(values, s)
		}
	case []interface{}:
		for _, one := range vv {
			if s := strings.TrimSpace(toString(one)); s != "" {
				values = append(values, s)
			}
		}
	default:
		if s := strings.TrimSpace(toString(v)); s != "" {
			values = append(values, s)
		}
	}
	if len(values) == 0 {
		return ""
	}

	seen := make(map[string]struct{}, len(values))
	names := make([]string, 0, len(values))
	for _, one := range values {
		name := adRoleNameFromDN(one)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return strings.Join(names, "、")
}

func adRoleNameFromDN(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	upper := strings.ToUpper(text)
	if idx := strings.Index(upper, "CN="); idx >= 0 {
		part := text[idx+3:]
		if cut := strings.Index(part, ","); cut >= 0 {
			part = part[:cut]
		}
		if s := strings.TrimSpace(part); s != "" {
			return s
		}
	}
	if cut := strings.Index(text, ","); cut >= 0 {
		text = text[:cut]
	}
	return strings.TrimSpace(text)
}

func cellAt(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}

func adSearchUsers(client *http.Client, p map[string]interface{}) projectResult {
	search := strings.TrimSpace(toString(p["search_name"]))
	if search == "" {
		return projectResult{OK: false, Message: "查询用户失败", Error: "必填项不能为空"}
	}

	searchLower := strings.ToLower(search)
	data, err := adSearchRaw(client, search)
	if err != nil {
		return projectResult{OK: false, Message: "查询用户失败", Error: err.Error()}
	}

	items := make([]map[string]interface{}, 0)
	logEntries := make([]string, 0)
	for _, one := range toSlice(data["message"]) {
		m, ok := one.(map[string]interface{})
		if !ok {
			continue
		}

		account := strings.TrimSpace(toString(m["sAMAccountName"]))
		if account == "" {
			continue
		}
		if strings.HasSuffix(account, "$") {
			continue
		}

		dn := strings.TrimSpace(toString(m["distinguishedName"]))
		if strings.Contains(strings.ToLower(dn), "cn=computers,") {
			continue
		}

		displayName := strings.TrimSpace(toString(m["displayName"]))
		desc := ""
		d := toSlice(m["description"])
		if len(d) > 0 {
			desc = strings.TrimSpace(toString(d[0]))
		}
		roleText := adRoleTextFromMessage(m)

		if !strings.Contains(strings.ToLower(account), searchLower) &&
			!strings.Contains(strings.ToLower(displayName), searchLower) &&
			!strings.Contains(strings.ToLower(desc), searchLower) {
			continue
		}

		items = append(items, map[string]interface{}{
			"account":     account,
			"displayName": displayName,
			"description": desc,
			"roles":       roleText,
			"dn":          dn,
		})
		logEntries = append(logEntries, fmt.Sprintf("账号：%s\n显示名称：%s\n描述：%s\n路径：%s", account, displayName, desc, dn))
	}

	var logBuilder strings.Builder
	for idx, one := range logEntries {
		if logBuilder.Len() > 0 {
			logBuilder.WriteString("\n\n")
		}
		logBuilder.WriteString(one)
		emitProgress(p, one, idx+1, len(logEntries))
	}

	logText := strings.TrimSpace(logBuilder.String())
	if logText == "" {
		logText = "未查询到相关搜索信息"
	}

	return projectResult{
		OK:      true,
		Message: fmt.Sprintf("查询完成，共 %d 条", len(items)),
		Data: map[string]interface{}{
			"items":    items,
			"log_text": logText,
		},
	}
}

func adResetPassword(client *http.Client, p map[string]interface{}) projectResult {
	name := strings.TrimSpace(toString(p["name"]))
	password := strings.TrimSpace(toString(p["password"]))
	if name == "" {
		return projectResult{OK: false, Message: "重置密码失败", Error: "必填项不能为空"}
	}
	if password == "" {
		return projectResult{OK: false, Message: "重置密码失败", Error: "新密码不能为空"}
	}
	if !isValidStrongPassword(password) {
		return projectResult{OK: false, Message: "重置密码失败", Error: "密码至少8位，且包含大小写字母和数字"}
	}
	dn, err := adFindDN(client, name)
	if err != nil {
		return projectResult{OK: false, Message: "重置密码失败", Error: err.Error()}
	}
	if dn == "" {
		return projectResult{OK: false, Message: "重置密码失败", Error: "用户不存在"}
	}
	payload := url.Values{}
	payload.Set("distinguishedName", dn)
	payload.Set("newpassword", password)
	if toBoolDefault(p["pwd_last_set"], true) {
		payload.Set("pwdLastSet", "true")
	} else {
		payload.Set("pwdLastSet", "false")
	}
	resp, err := postForm(client, adEndpoint("resetUserPassword/"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "重置密码失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "重置密码失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "重置密码失败", Error: err.Error()}
	}
	if toBool(data["isSuccess"]) {
		return projectResult{OK: true, Message: "重置密码成功", Data: map[string]interface{}{"raw": data, "log_text": "重置密码成功"}}
	}
	return projectResult{OK: false, Message: "重置密码失败", Error: "执行失败", Data: map[string]interface{}{"raw": data}}
}

func adUnlockUser(client *http.Client, p map[string]interface{}) projectResult {
	name := strings.TrimSpace(toString(p["name"]))
	if name == "" {
		return projectResult{OK: false, Message: "解锁失败", Error: "必填项不能为空"}
	}
	dn, err := adFindDN(client, name)
	if err != nil {
		return projectResult{OK: false, Message: "解锁失败", Error: err.Error()}
	}
	if dn == "" {
		return projectResult{OK: false, Message: "解锁失败", Error: "用户不存在"}
	}
	payload := url.Values{}
	payload.Set("sAMAccountName", name)
	resp, err := postForm(client, adEndpoint("unLockuser/"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "解锁失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "解锁失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "解锁失败", Error: err.Error()}
	}
	if toBool(data["isSuccess"]) {
		return projectResult{OK: true, Message: "解锁成功", Data: map[string]interface{}{"raw": data, "log_text": "解锁用户成功"}}
	}
	return projectResult{OK: false, Message: "解锁失败", Error: "执行失败", Data: map[string]interface{}{"raw": data}}
}

func adModifyDescription(client *http.Client, p map[string]interface{}) projectResult {
	name := strings.TrimSpace(toString(p["name"]))
	desc := toString(p["description"])
	if name == "" {
		return projectResult{OK: false, Message: "修改描述失败", Error: "必填项不能为空"}
	}
	if dn, _ := adFindDN(client, name); dn == "" {
		return projectResult{OK: false, Message: "修改描述失败", Error: "用户不存在"}
	}
	q := url.Values{}
	q.Set("CountName", name)
	q.Set("Attributes", "description")
	q.Set("ChangeMessage", desc)
	resp, err := client.Get(adEndpoint("api/ChangeUserMessage/") + "?" + q.Encode())
	if err != nil {
		return projectResult{OK: false, Message: "修改描述失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "修改描述失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "修改描述失败", Error: err.Error()}
	}
	if toBool(data["isSuccess"]) {
		return projectResult{OK: true, Message: "修改描述成功", Data: map[string]interface{}{"raw": data, "log_text": fmt.Sprintf("%s修改描述成功", name)}}
	}
	return projectResult{OK: false, Message: "修改描述失败", Error: "执行失败", Data: map[string]interface{}{"raw": data}}
}

func adModifyName(client *http.Client, p map[string]interface{}) projectResult {
	name := strings.TrimSpace(toString(p["name"]))
	cn := strings.TrimSpace(toString(p["cn"]))
	if name == "" {
		return projectResult{OK: false, Message: "修改姓名失败", Error: "必填项不能为空"}
	}
	if cn == "" {
		return projectResult{OK: false, Message: "修改姓名失败", Error: "姓名不能为空"}
	}
	dn, err := adFindDN(client, name)
	if err != nil {
		return projectResult{OK: false, Message: "修改姓名失败", Error: err.Error()}
	}
	if dn == "" {
		return projectResult{OK: false, Message: "修改姓名失败", Error: "用户不存在"}
	}
	payload := url.Values{}
	payload.Set("distinguishedName", dn)
	payload.Set("cn", cn)
	payload.Set("sn", toString(p["sn"]))
	payload.Set("givenName", toString(p["given_name"]))
	payload.Set("displayName", cn)
	payload.Set("userPrincipalName", fmt.Sprintf("%s@vdesktop.sunline.cn", name))
	payload.Set("sAMAccountName", name)
	payload.Set("objectClass", "top,person,organizationalPerson,user")
	resp, err := postForm(client, adEndpoint("setRenameObject/"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "修改姓名失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "修改姓名失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "修改姓名失败", Error: err.Error()}
	}
	if toBool(data["isSuccess"]) {
		return projectResult{OK: true, Message: "修改姓名成功", Data: map[string]interface{}{"raw": data, "log_text": fmt.Sprintf("%s修改姓名成功", name)}}
	}
	return projectResult{OK: false, Message: "修改姓名失败", Error: "执行失败", Data: map[string]interface{}{"raw": data}}
}

func adDeleteUser(client *http.Client, p map[string]interface{}) projectResult {
	name := strings.TrimSpace(toString(p["name"]))
	if name == "" {
		return projectResult{OK: false, Message: "删除用户失败", Error: "必填项不能为空"}
	}
	dn, err := adFindDN(client, name)
	if err != nil {
		return projectResult{OK: false, Message: "删除用户失败", Error: err.Error()}
	}
	if dn == "" {
		return projectResult{OK: false, Message: "删除用户失败", Error: "用户不存在"}
	}
	payload := url.Values{}
	payload.Set("dn", dn)
	resp, err := postForm(client, adEndpoint("delObject/"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "删除用户失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "删除用户失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "删除用户失败", Error: err.Error()}
	}
	if toBool(data["isSuccess"]) {
		return projectResult{OK: true, Message: "删除用户成功", Data: map[string]interface{}{"raw": data, "log_text": "删除用户成功"}}
	}
	return projectResult{OK: false, Message: "删除用户失败", Error: "执行失败", Data: map[string]interface{}{"raw": data}}
}

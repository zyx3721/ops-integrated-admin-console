package project

import (
	"bytes"
	"crypto/aes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type printCtx struct {
	client    *http.Client
	csrfToken string
}

type printSearchItem struct {
	Name     string `json:"name"`
	Fullname string `json:"fullname"`
	Email    string `json:"email"`
	Dept     string `json:"dept"`
}

var printRoleNameToID = map[string]string{
	"彩色权限": "13a8c61c6888a4c",
	"报表":   "36b238261872cd10208",
	"管理员":  "7d9bfe7cd65a29",
	"黑白权限": "12483a1e79473e4",
}

func printLogin(username, password string) (*printCtx, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Timeout: 25 * time.Second, Jar: jar}
	csrf0 := "cf003610-37c6-4963-916f-d9f6ecd6affd"
	preTok, _ := printOnceToken(csrf0)
	preURL := printEndpoint("isEnableVerifyCode")
	preReq, err := http.NewRequest(http.MethodGet, preURL, nil)
	if err != nil {
		return nil, err
	}
	// Keep Python behavior: token is already quoted once and query encoding quotes it again.
	q := preReq.URL.Query()
	q.Set("csrftoken", preTok)
	preReq.URL.RawQuery = q.Encode()
	resp, err := client.Do(preReq)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pre login http=%d body=%s", resp.StatusCode, truncate(string(body), 200))
	}
	preData, err := decodeRespJSON(resp)
	if err != nil {
		return nil, err
	}
	if toInt(preData["code"]) != 0 {
		return nil, fmt.Errorf("pre login failed: %v", preData)
	}
	csrf := resp.Header.Get("csrftoken")
	if csrf == "" {
		return nil, errors.New("missing csrf token")
	}
	nameEnc, _ := printEncryptAES(username)
	pwdEnc, _ := printEncryptAES(password)
	loginTok, _ := printOnceToken(csrf)
	payload := url.Values{}
	payload.Set("csrftoken", loginTok)
	payload.Set("name", nameEnc)
	payload.Set("pwd", pwdEnc)
	payload.Set("unlockDevice", "false")
	payload.Set("forceLogin", "false")
	lr, err := postForm(client, printEndpoint("login"), payload)
	if err != nil {
		return nil, err
	}
	if lr.StatusCode != http.StatusOK {
		defer lr.Body.Close()
		body, _ := io.ReadAll(lr.Body)
		return nil, fmt.Errorf("login http=%d body=%s", lr.StatusCode, truncate(string(body), 200))
	}
	ld, err := decodeRespJSON(lr)
	if err != nil {
		return nil, err
	}
	if toInt(ld["code"]) != 0 {
		return nil, fmt.Errorf("login failed: %s", toString(ld["msg"]))
	}
	return &printCtx{client: client, csrfToken: csrf}, nil
}

func printOperate(ctx *printCtx, action string, p map[string]interface{}) projectResult {
	switch action {
	case "add_user":
		return printAddUser(ctx, p)
	case "search_user":
		return printSearchUser(ctx, p)
	case "get_user":
		return printGetUser(ctx, p)
	case "reset_password":
		return printResetPassword(ctx, p)
	case "modify_user":
		return printModifyUser(ctx, p)
	case "delete_user":
		return printDeleteUser(ctx, p)
	default:
		return projectResult{OK: false, Message: "不支持的打印管理操作", Error: "不支持的操作"}
	}
}

func printDeptID(ctx *printCtx, section string) (string, error) {
	tok, _ := printOnceToken(ctx.csrfToken)
	payload := url.Values{}
	payload.Set("csrftoken", tok)
	payload.Set("flag", "dept")
	payload.Set("page", "1")
	payload.Set("pagesize", "500")
	resp, err := postForm(ctx.client, printEndpoint("api/right/dept/queryTable"), payload)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("dept http=%d", resp.StatusCode)
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return "", err
	}
	for _, one := range toSlice(data["data"]) {
		m, ok := one.(map[string]interface{})
		if ok && toString(m["name"]) == section {
			return toString(m["id"]), nil
		}
	}
	return "", nil
}

func printSearchUserRaw(ctx *printCtx, key, value string) (map[string]interface{}, error) {
	tok, _ := printOnceToken(ctx.csrfToken)
	payload := url.Values{}
	payload.Set("csrftoken", tok)
	payload.Set("pageSizeNum", "500")
	if key != "email" {
		payload.Set("userNameType", key)
	}
	payload.Set(key, value)
	resp, err := postForm(ctx.client, printEndpoint("api/right/user/query"), payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("query http=%d", resp.StatusCode)
	}
	return decodeRespJSON(resp)
}

func printFindUser(ctx *printCtx, key, value string) (map[string]interface{}, error) {
	data, err := printSearchUserRaw(ctx, key, value)
	if err != nil {
		return nil, err
	}
	for _, one := range toSlice(data["data"]) {
		m, ok := one.(map[string]interface{})
		if !ok {
			continue
		}
		if key == "username" && toString(m["name"]) == value {
			return m, nil
		}
		if key == "fullname" && toString(m["fullname"]) == value {
			return m, nil
		}
		if key == "email" && toString(m["email"]) == value {
			return m, nil
		}
	}
	return nil, nil
}

func printAllowedSearchKey(v string) string {
	switch strings.TrimSpace(v) {
	case "username", "fullname", "email":
		return strings.TrimSpace(v)
	default:
		return "fullname"
	}
}

func printFieldValue(v interface{}) string {
	if m, ok := v.(map[string]interface{}); ok {
		if s := strings.TrimSpace(toString(m["value"])); s != "" {
			return s
		}
	}
	return strings.TrimSpace(toString(v))
}

func printRoleIDsFromNames(roleNames string) []string {
	roleIDs := make([]string, 0, 4)
	for _, one := range strings.Split(roleNames, "|") {
		name := strings.TrimSpace(one)
		if name == "" {
			continue
		}
		if id := strings.TrimSpace(printRoleNameToID[name]); id != "" {
			roleIDs = append(roleIDs, id)
		}
	}
	return roleIDs
}

func printNormalizeRoleIDs(v interface{}) []string {
	raw := make([]string, 0)
	switch vv := v.(type) {
	case string:
		raw = strings.Split(vv, ",")
	case []interface{}:
		for _, one := range vv {
			raw = append(raw, toString(one))
		}
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(raw))
	for _, one := range raw {
		id := strings.TrimSpace(one)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func printNormalizePathName(s string) string {
	s = strings.TrimSpace(s)
	for strings.Contains(s, `\\`) {
		s = strings.ReplaceAll(s, `\\`, `\`)
	}
	return s
}

func printAddUser(ctx *printCtx, p map[string]interface{}) projectResult {
	name := strings.TrimSpace(toString(p["name"]))
	fullname := strings.TrimSpace(toString(p["fullname"]))
	sex := strings.TrimSpace(toString(p["sex"]))
	password := strings.TrimSpace(toString(p["password"]))
	email := strings.TrimSpace(toString(p["email"]))
	section := strings.TrimSpace(toString(p["section"]))
	if name == "" || fullname == "" || sex == "" || password == "" || email == "" || section == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "必填项不能为空"}
	}
	if sex != "male" && sex != "female" && sex != "unknown" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "性别参数不正确"}
	}
	if !isValidEmail(email) {
		return projectResult{OK: false, Message: "新增用户失败", Error: "邮箱格式不正确"}
	}
	deptID, err := printDeptID(ctx, section)
	if err != nil {
		return projectResult{OK: false, Message: "新增用户失败", Error: err.Error()}
	}
	if deptID == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "未找到对应部门"}
	}
	tok, _ := printOnceToken(ctx.csrfToken)
	nameEnc, _ := printEncryptAES(name)
	fullEnc, _ := printEncryptAES(fullname)
	pwdEnc, _ := printEncryptAES(password)
	payload := url.Values{}
	payload.Set("csrftoken", tok)
	payload.Set("name", nameEnc)
	payload.Set("fullname", fullEnc)
	payload.Set("sex", sex)
	payload.Set("pwd", pwdEnc)
	payload.Set("pwd2", pwdEnc)
	payload.Set("email", email)
	payload.Set("status", "enabled")
	payload.Set("dept", deptID)
	payload.Set("defaultlang", "zh_CN")
	payload.Set("docsecuritylevel", "public")
	payload.Set("roleIds", "12483a1e79473e4")
	payload.Set("isauditor", "false")
	payload.Set("userAuthStr", "[]")
	resp, err := postForm(ctx.client, printEndpoint("api/right/user/save"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "新增用户失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "新增用户失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "新增用户失败", Error: err.Error()}
	}
	if toInt(data["code"]) == 0 {
		return projectResult{OK: true, Message: "新增用户成功", Data: map[string]interface{}{"raw": data, "log_text": fmt.Sprintf("添加打印机用户 %s 成功", name)}}
	}
	return projectResult{OK: false, Message: "新增用户失败", Error: toString(data["msg"]), Data: map[string]interface{}{"raw": data}}
}

func printSearchUser(ctx *printCtx, p map[string]interface{}) projectResult {
	key := printAllowedSearchKey(toStringDefault(p["search_key"], "fullname"))
	value := strings.TrimSpace(toString(p["search_content"]))
	if value == "" {
		return projectResult{OK: false, Message: "查询用户失败", Error: "查询值不能为空"}
	}
	data, err := printSearchUserRaw(ctx, key, value)
	if err != nil {
		return projectResult{OK: false, Message: "查询用户失败", Error: err.Error()}
	}
	items := make([]printSearchItem, 0)
	var logBuilder strings.Builder
	for _, one := range toSlice(data["data"]) {
		if m, ok := one.(map[string]interface{}); ok {
			item := printSearchItem{
				Name:     toString(m["name"]),
				Fullname: toString(m["fullname"]),
				Email:    toString(m["email"]),
				Dept:     printNormalizePathName(toString(m["dept.name"])),
			}
			items = append(items, item)

			roleName := strings.TrimSpace(toString(m["roleNames"]))
			if roleName == "" {
				roleName = "-"
			}

			if logBuilder.Len() > 0 {
				logBuilder.WriteString("\n\n")
			}
			logBuilder.WriteString("用户名：")
			logBuilder.WriteString(item.Name)
			logBuilder.WriteString("\n姓名：")
			logBuilder.WriteString(item.Fullname)
			logBuilder.WriteString("\n邮箱：")
			logBuilder.WriteString(item.Email)
			logBuilder.WriteString("\n部门：")
			logBuilder.WriteString(item.Dept)
			logBuilder.WriteString("\n角色：")
			logBuilder.WriteString(roleName)
			emitProgress(p, fmt.Sprintf("匹配到打印用户：%s", item.Name), len(items), 0)
		}
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

func printGetUser(ctx *printCtx, p map[string]interface{}) projectResult {
	key := printAllowedSearchKey(toStringDefault(p["search_key"], "fullname"))
	value := strings.TrimSpace(toString(p["search_content"]))
	if value == "" {
		return projectResult{OK: false, Message: "查询用户失败", Error: "查询值不能为空"}
	}
	u, err := printFindUser(ctx, key, value)
	if err != nil {
		return projectResult{OK: false, Message: "查询用户失败", Error: err.Error()}
	}
	if u == nil {
		return projectResult{OK: false, Message: "查询用户失败", Error: "用户不存在"}
	}
	roleNames := toString(u["roleNames"])
	roleIDs := printRoleIDsFromNames(roleNames)
	return projectResult{
		OK:      true,
		Message: "查询成功",
		Data: map[string]interface{}{
			"item": map[string]interface{}{
				"id":         toString(u["id"]),
				"name":       toString(u["name"]),
				"fullname":   toString(u["fullname"]),
				"sex":        printFieldValue(u["sex"]),
				"status":     printFieldValue(u["status"]),
				"email":      toString(u["email"]),
				"section":    printNormalizePathName(toString(u["dept.name"])),
				"role_names": roleNames,
				"role_ids":   roleIDs,
			},
		},
	}
}

func printResetPassword(ctx *printCtx, p map[string]interface{}) projectResult {
	key := printAllowedSearchKey(toStringDefault(p["search_key"], "fullname"))
	value := strings.TrimSpace(toString(p["search_content"]))
	if value == "" {
		return projectResult{OK: false, Message: "重置密码失败", Error: "查询值不能为空"}
	}
	password := toStringDefault(p["password"], "123")
	u, err := printFindUser(ctx, key, value)
	if err != nil {
		return projectResult{OK: false, Message: "重置密码失败", Error: err.Error()}
	}
	if u == nil {
		return projectResult{OK: false, Message: "重置密码失败", Error: "用户不存在"}
	}
	tok, _ := printOnceToken(ctx.csrfToken)
	pwdEnc, _ := printEncryptAES(password)
	payload := url.Values{}
	payload.Set("csrftoken", tok)
	payload.Set("userId", toString(u["id"]))
	payload.Set("sendEmail", "false")
	payload.Set("pwd", pwdEnc)
	resp, err := postForm(ctx.client, printEndpoint("api/right/user/setDefPwd"), payload)
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
	if toInt(data["code"]) == 0 {
		return projectResult{OK: true, Message: "重置密码成功", Data: map[string]interface{}{"raw": data, "log_text": "重置密码成功"}}
	}
	return projectResult{OK: false, Message: "重置密码失败", Error: toString(data["msg"]), Data: map[string]interface{}{"raw": data}}
}

func printModifyUser(ctx *printCtx, p map[string]interface{}) projectResult {
	key := printAllowedSearchKey(toStringDefault(p["search_key"], "fullname"))
	value := strings.TrimSpace(toString(p["search_content"]))
	userID := strings.TrimSpace(toString(p["user_id"]))
	oriEmail := strings.TrimSpace(toString(p["ori_email"]))

	name := strings.TrimSpace(toString(p["name"]))
	fullname := strings.TrimSpace(toString(p["fullname"]))
	sex := strings.TrimSpace(toString(p["sex"]))
	status := strings.TrimSpace(toString(p["status"]))
	email := strings.TrimSpace(toString(p["email"]))
	section := strings.TrimSpace(toString(p["section"]))
	roleIDs := printNormalizeRoleIDs(p["roles"])

	// Keep backward compatibility: if user_id is not provided, locate user by search key/value.
	if userID == "" {
		if value == "" {
			return projectResult{OK: false, Message: "修改用户失败", Error: "查询值不能为空"}
		}
		u, err := printFindUser(ctx, key, value)
		if err != nil {
			return projectResult{OK: false, Message: "修改用户失败", Error: err.Error()}
		}
		if u == nil {
			return projectResult{OK: false, Message: "修改用户失败", Error: "用户不存在"}
		}
		userID = toString(u["id"])
		if name == "" {
			name = strings.TrimSpace(toString(u["name"]))
		}
		if fullname == "" {
			fullname = strings.TrimSpace(toString(u["fullname"]))
		}
		if sex == "" {
			sex = printFieldValue(u["sex"])
		}
		if status == "" {
			status = printFieldValue(u["status"])
		}
		if email == "" {
			email = strings.TrimSpace(toString(u["email"]))
		}
		if section == "" {
			section = strings.TrimSpace(toString(u["dept.name"]))
		}
		if oriEmail == "" {
			oriEmail = strings.TrimSpace(toString(u["email"]))
		}
		if len(roleIDs) == 0 {
			roleIDs = printRoleIDsFromNames(toString(u["roleNames"]))
		}
	}

	if name == "" || fullname == "" || sex == "" || status == "" || section == "" {
		return projectResult{OK: false, Message: "修改用户失败", Error: "必填项不能为空"}
	}
	if sex != "male" && sex != "female" && sex != "unknown" {
		return projectResult{OK: false, Message: "修改用户失败", Error: "性别参数不正确"}
	}
	if status != "enabled" && status != "disabled" {
		return projectResult{OK: false, Message: "修改用户失败", Error: "状态参数不正确"}
	}
	if email != "" && !isValidEmail(email) {
		return projectResult{OK: false, Message: "修改用户失败", Error: "邮箱格式不正确"}
	}
	if len(roleIDs) == 0 {
		return projectResult{OK: false, Message: "修改用户失败", Error: "角色不能为空"}
	}
	deptID, err := printDeptID(ctx, section)
	if err != nil {
		return projectResult{OK: false, Message: "修改用户失败", Error: err.Error()}
	}
	if deptID == "" {
		return projectResult{OK: false, Message: "修改用户失败", Error: "未找到对应部门"}
	}

	tok, _ := printOnceToken(ctx.csrfToken)
	nameEnc, _ := printEncryptAES(name)
	fullEnc, _ := printEncryptAES(fullname)
	payload := url.Values{}
	payload.Set("csrftoken", tok)
	payload.Set("name", nameEnc)
	payload.Set("fullname", fullEnc)
	payload.Set("sex", sex)
	payload.Set("email", email)
	payload.Set("status", status)
	payload.Set("dept", deptID)
	payload.Set("defaultlang", "zh_CN")
	payload.Set("docsecuritylevel", "public")
	payload.Set("roleIds", strings.Join(roleIDs, ","))
	payload.Set("isauditor", "false")
	payload.Set("id", userID)
	payload.Set("userAuthStr", "[]")
	payload.Set("oriEmail", oriEmail)
	resp, err := postForm(ctx.client, printEndpoint("api/right/user/save"), payload)
	if err != nil {
		return projectResult{OK: false, Message: "修改用户失败", Error: err.Error()}
	}
	if resp.StatusCode != http.StatusOK {
		return projectResult{OK: false, Message: "修改用户失败", Error: "HTTP请求失败"}
	}
	data, err := decodeRespJSON(resp)
	if err != nil {
		return projectResult{OK: false, Message: "修改用户失败", Error: err.Error()}
	}
	if toInt(data["code"]) == 0 {
		return projectResult{OK: true, Message: "修改用户成功", Data: map[string]interface{}{"raw": data, "log_text": fmt.Sprintf("修改打印机用户 %s 成功", name)}}
	}
	return projectResult{OK: false, Message: "修改用户失败", Error: toString(data["msg"]), Data: map[string]interface{}{"raw": data}}
}

func printDeleteUser(ctx *printCtx, p map[string]interface{}) projectResult {
	key := printAllowedSearchKey(toStringDefault(p["search_key"], "fullname"))
	value := strings.TrimSpace(toString(p["search_content"]))
	if value == "" {
		return projectResult{OK: false, Message: "删除用户失败", Error: "查询值不能为空"}
	}
	u, err := printFindUser(ctx, key, value)
	if err != nil {
		return projectResult{OK: false, Message: "删除用户失败", Error: err.Error()}
	}
	if u == nil {
		return projectResult{OK: false, Message: "删除用户失败", Error: "用户不存在"}
	}
	tok, _ := printOnceToken(ctx.csrfToken)
	payload := url.Values{}
	payload.Set("csrftoken", tok)
	payload.Set("id", toString(u["id"]))
	resp, err := postForm(ctx.client, printEndpoint("api/right/user/delete"), payload)
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
	if toInt(data["code"]) == 0 {
		return projectResult{OK: true, Message: "删除用户成功", Data: map[string]interface{}{"raw": data, "log_text": "删除用户成功"}}
	}
	return projectResult{OK: false, Message: "删除用户失败", Error: toString(data["msg"]), Data: map[string]interface{}{"raw": data}}
}

func printOnceToken(csrf string) (string, error) {
	ts := strconv.FormatInt(time.Now().UnixNano()/int64(time.Millisecond), 10)
	enc, err := aesECBEncryptBase64(csrf+"_"+ts, "abcdefgabcdefg12")
	if err != nil {
		return "", err
	}
	return url.QueryEscape(enc), nil
}

func printEncryptAES(text string) (string, error) {
	return aesECBEncryptBase64(text, "abcdefgabcdefg12")
}

func aesECBEncryptBase64(plaintext, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	bs := block.BlockSize()
	data := pkcs7Pad([]byte(plaintext), bs)
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += bs {
		block.Encrypt(out[i:i+bs], data[i:i+bs])
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - (len(data) % blockSize)
	if pad == 0 {
		pad = blockSize
	}
	padding := bytes.Repeat([]byte{byte(pad)}, pad)
	return append(data, padding...)
}

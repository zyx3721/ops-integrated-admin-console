package project

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func vpnLogin(username, password, host string, port int) (*ssh.Client, error) {
	cfg := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	return ssh.Dial("tcp", fmt.Sprintf("%s:%d", host, port), cfg)
}

func vpnLoginFromParams(p map[string]interface{}, host string) (*ssh.Client, error) {
	account := strings.TrimSpace(toString(p["__vpn_account"]))
	password := strings.TrimSpace(toString(p["__vpn_password"]))
	if account == "" || password == "" {
		return nil, fmt.Errorf("VPN 账号或密码未配置")
	}
	return vpnLogin(account, password, host, 22)
}

func vpnRun(client *ssh.Client, command string) (string, error) {
	s, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()

	if err = s.RequestPty("vt100", 40, 120, ssh.TerminalModes{}); err != nil {
		return "", err
	}
	stdin, err := s.StdinPipe()
	if err != nil {
		return "", err
	}
	stdout, err := s.StdoutPipe()
	if err != nil {
		return "", err
	}
	stderr, err := s.StderrPipe()
	if err != nil {
		return "", err
	}
	if err = s.Shell(); err != nil {
		return "", err
	}

	chunkCh := make(chan []byte, 128)
	readPipe := func(r io.Reader) {
		buf := make([]byte, 4096)
		for {
			n, er := r.Read(buf)
			if n > 0 {
				one := make([]byte, n)
				copy(one, buf[:n])
				chunkCh <- one
			}
			if er != nil {
				return
			}
		}
	}
	go readPipe(stdout)
	go readPipe(stderr)

	collect := func(quiet, maxWait time.Duration, handlePager bool) []byte {
		var out bytes.Buffer
		quietTimer := time.NewTimer(quiet)
		defer quietTimer.Stop()
		deadline := time.Now().Add(maxWait)
		for {
			if time.Now().After(deadline) {
				return out.Bytes()
			}
			select {
			case chunk := <-chunkCh:
				if handlePager && bytes.Contains(chunk, []byte("--More--")) {
					chunk = bytes.ReplaceAll(chunk, []byte("--More--"), []byte(""))
					_, _ = stdin.Write([]byte(" "))
				}
				_, _ = out.Write(chunk)
				if !quietTimer.Stop() {
					select {
					case <-quietTimer.C:
					default:
					}
				}
				quietTimer.Reset(quiet)
			case <-quietTimer.C:
				return out.Bytes()
			}
		}
	}

	_ = collect(200*time.Millisecond, 1500*time.Millisecond, false)
	if _, err = stdin.Write([]byte(command + "\n")); err != nil {
		return "", err
	}
	raw := collect(1200*time.Millisecond, 20*time.Second, true)
	return vpnDecodeOutput(raw), nil
}

func vpnDecodeOutput(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	decoded, _, err := transform.String(simplifiedchinese.GB18030.NewDecoder(), string(raw))
	if err == nil {
		return decoded
	}
	reader := transform.NewReader(bytes.NewReader(raw), simplifiedchinese.GB18030.NewDecoder())
	b, readErr := io.ReadAll(reader)
	if readErr == nil {
		return string(b)
	}
	return string(raw)
}

type vpnSearchItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Mail        string `json:"mail"`
	Status      string `json:"status"`
	StatusText  string `json:"status_text"`
}

var vpnErrorCodeRegex = regexp.MustCompile(`-\d{4,}`)
var vpnNameRegex = regexp.MustCompile(`name\s+(\S+)`)
var vpnDescRegex = regexp.MustCompile(`invalid\s+\S+\s+description\s+(.*?)\s+group`)
var vpnGroupRegex = regexp.MustCompile(`group\s+(\S+)\^`)
var vpnMailRegex = regexp.MustCompile(`mail\s+(\S+)`)
var vpnInvalidRegex = regexp.MustCompile(`invalid\s+(\S+)`)

var vpnSectionAll = []string{
	"default^root",
	"数金总部^root",
	"数据总部^root",
	"长亮控股^root",
	"长亮合度^root",
	"长亮金服^root",
	"Manager^root",
	"temp^root",
	"长亮科技^root",
	"共享服务中心^长亮科技^root",
	"集团解决方案部^长亮科技^root",
	"人力资源中心^长亮科技^root",
	"信息服务中心^长亮科技^root",
	"财务中心^长亮科技^root",
	"运营中心^长亮科技^root",
	"咨询业务中心^长亮科技^root",
	"集团产品发展部^长亮科技^root",
	"干部部^长亮科技^root",
	"北京运营中心^长亮科技^root",
	"市场部^长亮科技^root",
	"董事会^长亮科技^root",
	"集团总裁办^长亮科技^root",
	"总裁办公室^长亮科技^root",
	"研发中心^长亮科技^root",
	"销售总部^长亮科技^root",
	"内部审计部^长亮科技^root",
	"公共关系部^长亮科技^root",
	"战略规划部^长亮科技^root",
	"健康督导办公室^长亮科技^root",
	"税务部^长亮科技^root",
	"集团项目管理部^长亮科技^root",
	"研发体系^长亮科技^root",
	"董事会办公室^长亮科技^root",
	"战略发展部^长亮科技^root",
	"浙农信项目^root",
}

var vpnSectionSet = func() map[string]struct{} {
	m := make(map[string]struct{}, len(vpnSectionAll))
	for _, one := range vpnSectionAll {
		m[one] = struct{}{}
	}
	return m
}()

func vpnSectionExists(section string) bool {
	_, ok := vpnSectionSet[strings.TrimSpace(section)]
	return ok
}

func vpnNormalizeSection(section string) string {
	sec := strings.TrimSpace(section)
	if sec == "" {
		return "default^root"
	}
	if vpnSectionExists(sec) {
		return sec
	}
	if vpnSectionExists(sec + "^root") {
		return sec + "^root"
	}
	return "default^root"
}

func vpnStatusToInvalid(status string) string {
	switch strings.TrimSpace(strings.ToLower(status)) {
	case "enabled", "enable", "no":
		return "no"
	default:
		return "yes"
	}
}

func vpnInvalidToStatus(invalid string) (string, string) {
	if strings.TrimSpace(strings.ToLower(invalid)) == "no" {
		return "enabled", "启用"
	}
	return "disabled", "禁用"
}

func vpnIsUserNotFound(out string) bool {
	l := strings.ToLower(out)
	return strings.Contains(out, "-24501") ||
		strings.Contains(out, "不存") ||
		strings.Contains(l, "not exist") ||
		strings.Contains(l, "notfound") ||
		strings.Contains(l, "no such user")
}

func vpnIsUserExists(out string) bool {
	l := strings.ToLower(out)
	return strings.Contains(out, "已存") || strings.Contains(out, "存在") || strings.Contains(l, "already exist")
}

func vpnOutputLooksError(out string) bool {
	l := strings.ToLower(out)
	return vpnErrorCodeRegex.MatchString(out) || strings.Contains(l, "error") || strings.Contains(l, "failed")
}

func vpnDeleteLooksSuccess(out string) bool {
	l := strings.ToLower(out)
	if strings.TrimSpace(out) == "" {
		return false
	}
	if vpnIsUserNotFound(out) {
		return false
	}
	return !strings.Contains(l, "error") && !strings.Contains(l, "failed")
}

func vpnCleanDescription(desc string) string {
	d := strings.TrimSpace(desc)
	d = strings.ReplaceAll(d, "'", "")
	return d
}

func vpnDisplayGroup(group string) string {
	g := strings.TrimSpace(group)
	lower := strings.ToLower(g)
	if strings.HasSuffix(lower, "^root") && len(g) >= 5 {
		g = g[:len(g)-5]
	}
	return strings.TrimSpace(strings.TrimSuffix(g, "^"))
}

func vpnFormatSearchItemLog(item vpnSearchItem) string {
	return fmt.Sprintf(
		"名称：%s\n描述：%s\n所属父组：%s\n邮箱：%s\n状态：%s",
		item.Name,
		item.Description,
		item.Group,
		item.Mail,
		item.StatusText,
	)
}

func vpnDeleteOneUser(client *ssh.Client, username string, p map[string]interface{}) (bool, bool, string, error) {
	cmd := fmt.Sprintf("aaaa user user delete index-key name index-value %s", username)
	tryDelete := func(cli *ssh.Client) (string, error) {
		return vpnRun(cli, cmd)
	}

	out, err := tryDelete(client)
	if err != nil && strings.TrimSpace(out) == "" {
		out, err = tryDelete(client)
	}
	if err != nil && strings.TrimSpace(out) == "" {
		reopenCli, loginErr := vpnLoginFromParams(p, runtimeCfg.VPNSshAddr)
		if loginErr == nil {
			out, err = tryDelete(reopenCli)
			_ = reopenCli.Close()
		}
	}

	if vpnIsUserNotFound(out) {
		return false, true, out, err
	}
	if vpnDeleteLooksSuccess(out) {
		return true, false, out, err
	}
	return false, false, out, err
}

func vpnBuildSearchResult(out string) ([]vpnSearchItem, string) {
	text := strings.ReplaceAll(strings.ReplaceAll(out, "\r\n", ""), "\n", "")
	names := vpnNameRegex.FindAllStringSubmatch(text, -1)
	descs := vpnDescRegex.FindAllStringSubmatch(text, -1)
	groups := vpnGroupRegex.FindAllStringSubmatch(text, -1)
	mails := vpnMailRegex.FindAllStringSubmatch(text, -1)
	invalids := vpnInvalidRegex.FindAllStringSubmatch(text, -1)

	n := minInt(len(names), len(descs), len(groups), len(mails), len(invalids))
	if n <= 0 {
		return []vpnSearchItem{}, ""
	}

	items := make([]vpnSearchItem, 0, n)
	var b strings.Builder
	for i := 0; i < n; i++ {
		status, statusText := vpnInvalidToStatus(toString(invalids[i][1]))
		item := vpnSearchItem{
			Name:        strings.TrimSpace(toString(names[i][1])),
			Description: strings.TrimSpace(toString(descs[i][1])),
			Group:       vpnDisplayGroup(toString(groups[i][1])),
			Mail:        strings.TrimSpace(toString(mails[i][1])),
			Status:      status,
			StatusText:  statusText,
		}
		items = append(items, item)
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("名称：" + item.Name)
		b.WriteString("\n描述：" + item.Description)
		b.WriteString("\n所属父组：" + item.Group)
		b.WriteString("\n邮箱：" + item.Mail)
		b.WriteString("\n状态：" + item.StatusText)
	}
	return items, b.String()
}

func vpnOperate(client *ssh.Client, action string, p map[string]interface{}) projectResult {
	switch action {
	case "add_user":
		return vpnAddUser(client, p)
	case "search_user":
		return vpnSearchUser(client, p)
	case "modify_password":
		return vpnModifyPassword(client, p)
	case "modify_status":
		return vpnModifyStatus(client, p)
	case "delete_users":
		return vpnDeleteUsers(client, p)
	case "export_excel":
		return projectResult{OK: false, Message: "VPN 功能暂不支持导出 Excel", Error: "暂不支持导出功能"}
	default:
		return projectResult{OK: false, Message: "不支持的VPN操作", Error: "不支持的操作"}
	}
}

func vpnAddUser(client *ssh.Client, p map[string]interface{}) projectResult {
	n := strings.TrimSpace(toString(p["vpn_user"]))
	sec := vpnNormalizeSection(toString(p["section"]))
	pwd := strings.TrimSpace(toString(p["passwd"]))
	desc := strings.TrimSpace(toString(p["description"]))
	mail := strings.TrimSpace(toString(p["mail"]))
	status := strings.TrimSpace(toString(p["status"]))

	if n == "" || desc == "" || mail == "" || status == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: "必填项不能为空"}
	}
	if pwd == "" {
		pwd = randomPassword()
	}
	if !isValidStrongPassword(pwd) {
		return projectResult{OK: false, Message: "密码格式不符合要求", Error: "密码至少8位，且包含大小写字母和数字"}
	}
	if !isValidEmail(mail) {
		return projectResult{OK: false, Message: "新增用户失败", Error: "邮箱格式不正确"}
	}

	invalid := vpnStatusToInvalid(status)
	cmd := fmt.Sprintf("aaaa user user add name %s invalid %s group %s passwd %s description '%s' mail %s inherit-role yes", n, invalid, sec, pwd, vpnCleanDescription(desc), mail)
	out, err := vpnRun(client, cmd)
	if err != nil && strings.TrimSpace(out) == "" {
		return projectResult{OK: false, Message: "新增用户失败", Error: err.Error()}
	}
	if vpnIsUserExists(out) {
		return projectResult{OK: false, Message: "新增用户失败", Error: "用户名已存在", Data: map[string]interface{}{"output": out}}
	}
	if vpnOutputLooksError(out) {
		return projectResult{OK: false, Message: "新增用户失败", Error: "命令执行失败", Data: map[string]interface{}{"output": out}}
	}

	logText := fmt.Sprintf("用户名：%s\n初始密码：%s", n, pwd)
	return projectResult{OK: true, Message: "新增用户成功", Data: map[string]interface{}{"vpn_user": n, "passwd": pwd, "output": out, "log_text": logText}}
}

func vpnSearchUser(client *ssh.Client, p map[string]interface{}) projectResult {
	desc := strings.TrimSpace(toString(p["description"]))
	if desc == "" {
		return projectResult{OK: false, Message: "查询用户失败", Error: "描述不能为空"}
	}
	out, err := vpnRun(client, fmt.Sprintf("aaaa user user search key-word description show-type page key-value '%s'", vpnCleanDescription(desc)))
	if err != nil && strings.TrimSpace(out) == "" {
		return projectResult{OK: false, Message: "查询用户失败", Error: err.Error()}
	}
	if vpnOutputLooksError(out) && !strings.Contains(out, "name") {
		return projectResult{OK: false, Message: "查询用户失败", Error: "命令执行失败", Data: map[string]interface{}{"output": out}}
	}

	items, _ := vpnBuildSearchResult(out)
	if len(items) == 0 {
		return projectResult{OK: true, Message: "未查询到记录", Data: map[string]interface{}{"items": []vpnSearchItem{}, "raw": out, "log_text": "未查询到该用户的VPN信息"}}
	}
	logEntries := make([]string, 0, len(items))
	for i, item := range items {
		entry := vpnFormatSearchItemLog(item)
		logEntries = append(logEntries, entry)
		emitProgress(p, entry, i+1, len(items))
	}
	logText := strings.Join(logEntries, "\n\n")
	return projectResult{OK: true, Message: fmt.Sprintf("查询完成，共 %d 条", len(items)), Data: map[string]interface{}{"items": items, "raw": out, "log_text": logText}}
}

func vpnFindUserByDescription(client *ssh.Client, description string) (string, string, error) {
	desc := strings.TrimSpace(description)
	if desc == "" {
		return "", "", fmt.Errorf("描述不能为空")
	}
	out, err := vpnRun(client, fmt.Sprintf("aaaa user user search key-word description show-type page key-value '%s'", vpnCleanDescription(desc)))
	if err != nil && strings.TrimSpace(out) == "" {
		return "", out, err
	}
	items, _ := vpnBuildSearchResult(out)
	for _, item := range items {
		if strings.TrimSpace(item.Description) == desc {
			return strings.TrimSpace(item.Name), out, nil
		}
	}
	return "", out, fmt.Errorf("未找到匹配描述的用户")
}

func vpnModifyPassword(client *ssh.Client, p map[string]interface{}) projectResult {
	n := strings.TrimSpace(toString(p["vpn_user"]))
	desc := strings.TrimSpace(toString(p["description"]))
	searchOut := ""
	execClient := client

	if desc != "" {
		resolved, out, err := vpnFindUserByDescription(client, desc)
		searchOut = out
		if err != nil {
			return projectResult{OK: false, Message: "修改密码失败", Error: err.Error(), Data: map[string]interface{}{"output": out}}
		}
		n = resolved
		reopenCli, err := vpnLoginFromParams(p, runtimeCfg.VPNSshAddr)
		if err != nil {
			return projectResult{OK: false, Message: "VPN 重新登录失败", Error: err.Error(), Data: map[string]interface{}{"output": out}}
		}
		defer reopenCli.Close()
		execClient = reopenCli
	}
	if n == "" {
		return projectResult{OK: false, Message: "修改密码失败", Error: "描述不能为空"}
	}

	pwd := strings.TrimSpace(toString(p["passwd"]))
	if pwd == "" {
		pwd = randomPassword()
	}
	if !isValidStrongPassword(pwd) {
		return projectResult{OK: false, Message: "密码格式不符合要求", Error: "密码至少8位，且包含大小写字母和数字"}
	}

	out, err := vpnRun(execClient, fmt.Sprintf("aaaa user user modify-info passwd %s index-key name index-value %s", pwd, n))
	if err != nil && strings.TrimSpace(out) == "" {
		return projectResult{OK: false, Message: "修改密码失败", Error: err.Error()}
	}
	if strings.Contains(out, "-24501") {
		return projectResult{OK: false, Message: "修改失败", Error: "用户不存在", Data: map[string]interface{}{"output": out}}
	}
	if strings.Contains(out, "-24316") {
		return projectResult{OK: false, Message: "修改失败", Error: "新旧密码相同", Data: map[string]interface{}{"output": out}}
	}
	if strings.Contains(out, "-23204") {
		return projectResult{OK: false, Message: "修改失败", Error: "密码长度不足8位", Data: map[string]interface{}{"output": out}}
	}
	if vpnOutputLooksError(out) {
		return projectResult{OK: false, Message: "修改密码失败", Error: "命令执行失败", Data: map[string]interface{}{"output": out}}
	}

	logText := fmt.Sprintf("用户名：%s\n新密码：%s", n, pwd)
	return projectResult{OK: true, Message: "修改密码成功", Data: map[string]interface{}{"vpn_user": n, "passwd": pwd, "output": out, "search_output": searchOut, "log_text": logText}}
}

func vpnModifyStatus(client *ssh.Client, p map[string]interface{}) projectResult {
	n := strings.TrimSpace(toString(p["vpn_user"]))
	desc := strings.TrimSpace(toString(p["description"]))
	searchOut := ""
	execClient := client

	if desc != "" {
		resolved, out, err := vpnFindUserByDescription(client, desc)
		searchOut = out
		if err != nil {
			return projectResult{OK: false, Message: "修改状态失败", Error: err.Error(), Data: map[string]interface{}{"output": out}}
		}
		n = resolved
		reopenCli, err := vpnLoginFromParams(p, runtimeCfg.VPNSshAddr)
		if err != nil {
			return projectResult{OK: false, Message: "VPN 重新登录失败", Error: err.Error(), Data: map[string]interface{}{"output": out}}
		}
		defer reopenCli.Close()
		execClient = reopenCli
	}
	if n == "" {
		return projectResult{OK: false, Message: "修改状态失败", Error: "描述不能为空"}
	}

	status := strings.TrimSpace(toStringDefault(p["status"], "enabled"))
	invalid := vpnStatusToInvalid(status)
	_, statusText := vpnInvalidToStatus(invalid)

	out, err := vpnRun(execClient, fmt.Sprintf("aaaa user user modify-info invalid %s index-key name index-value %s", invalid, n))
	if err != nil && strings.TrimSpace(out) == "" {
		return projectResult{OK: false, Message: "修改状态失败", Error: err.Error()}
	}
	if strings.Contains(out, "-24501") {
		return projectResult{OK: false, Message: "修改状态失败", Error: "用户不存在", Data: map[string]interface{}{"output": out}}
	}
	if vpnOutputLooksError(out) {
		return projectResult{OK: false, Message: "修改状态失败", Error: "命令执行失败", Data: map[string]interface{}{"output": out}}
	}

	logText := fmt.Sprintf("用户名：%s\n状态：%s", n, statusText)
	return projectResult{OK: true, Message: "修改状态成功", Data: map[string]interface{}{"vpn_user": n, "status": status, "output": out, "search_output": searchOut, "log_text": logText}}
}

func vpnDeleteUsers(client *ssh.Client, p map[string]interface{}) projectResult {
	users := normalizeUsers(p["vpn_users"])
	if len(users) == 0 {
		users = normalizeUsers(p["vpn_user"])
	}
	if len(users) == 0 {
		users = normalizeUsers(p["vpn_users_text"])
	}
	if len(users) == 0 {
		return projectResult{OK: false, Message: "删除用户失败", Error: "用户名不能为空"}
	}

	items := make([]map[string]interface{}, 0, len(users))
	okCount := 0
	logs := make([]string, 0, len(users)+4)
	totalSteps := len(users)
	progressStep := 0

	for _, u := range users {
		ok, notFound, finalOut, finalErr := vpnDeleteOneUser(client, u, p)
		if ok {
			okCount++
			logs = append(logs, fmt.Sprintf("用户 %s 删除成功！", u))
		} else if notFound {
			logs = append(logs, fmt.Sprintf("删除失败！用户 %s 不存在！", u))
		} else {
			logs = append(logs, fmt.Sprintf("删除失败！用户 %s 删除异常！", u))
		}
		items = append(items, map[string]interface{}{"vpn_user": u, "ok": ok, "output": finalOut, "error": errString(finalErr)})
		progressStep++
		emitProgress(p, fmt.Sprintf("处理VPN用户：%s", u), progressStep, totalSteps)
	}

	data := map[string]interface{}{"items": items}

	if toBoolDefault(p["remote_firewall"], false) {
		fwConfigured := toBoolDefault(p["__vpn_fw_configured"], false)
		fwAccount := strings.TrimSpace(toString(p["__vpn_fw_account"]))
		fwPassword := strings.TrimSpace(toString(p["__vpn_fw_password"]))
		ritems := make([]map[string]interface{}, 0, len(users))
		rlogs := make([]string, 0, len(users)+4)

		if !fwConfigured || fwAccount == "" || fwPassword == "" {
			msg := "未配置防火墙账号密码，无法同步删除防火墙上的VPN账户"
			if strings.TrimSpace(toString(p["__vpn_fw_error"])) != "" {
				msg += "，原因：" + strings.TrimSpace(toString(p["__vpn_fw_error"]))
			}
			for _, u := range users {
				ritems = append(ritems, map[string]interface{}{"vpn_user": u, "ok": false, "output": "", "error": "防火墙凭据未配置"})
			}
			rlogs = append(rlogs, msg)
			data["remote_error"] = msg
		} else {
			rlogs = append(rlogs, "", "正在前往防火墙系统执行删除vpn用户....", "")
			rcli, loginErr := vpnLogin(fwAccount, fwPassword, runtimeCfg.FirewallSSHAddr, 22)
			if loginErr != nil {
					rlogs = append(rlogs, "远程防火墙系统失败！请检查用户名密码或访问权限！")
				for _, u := range users {
					ritems = append(ritems, map[string]interface{}{"vpn_user": u, "ok": false, "output": "", "error": errString(loginErr)})
				}
			} else {
				for _, u := range users {
					out, err := vpnRun(rcli, fmt.Sprintf("aaaa user user delete index-key name index-value %s", u))
					rok := vpnDeleteLooksSuccess(out)
					notFound := vpnIsUserNotFound(out)
					if rok {
						rlogs = append(rlogs, fmt.Sprintf("用户 %s 删除成功！", u))
					} else if notFound {
						rlogs = append(rlogs, fmt.Sprintf("删除失败！用户 %s 不存在！", u))
					} else {
						rlogs = append(rlogs, fmt.Sprintf("删除失败！用户 %s 删除异常！", u))
					}
					ritems = append(ritems, map[string]interface{}{"vpn_user": u, "ok": rok, "output": out, "error": errString(err)})
				}
				_ = rcli.Close()
			}
		}

		data["remote_items"] = ritems
		data["remote_log_text"] = strings.Join(rlogs, "\n")
		if len(rlogs) > 0 {
			logs = append(logs, rlogs...)
		}
	}

	logs = append(logs, "")
	data["log_text"] = strings.Join(logs, "\n")
	return projectResult{OK: true, Message: fmt.Sprintf("删除完成 %d/%d", okCount, len(users)), Data: data}
}

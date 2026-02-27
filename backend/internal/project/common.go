package project

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	mrand "math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ADAPIURL        string
	PrintAPIURL     string
	VPNSshAddr      string
	FirewallSSHAddr string
}

type Result struct {
	OK      bool                   `json:"ok"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error"`
}

type projectResult = Result

type ProgressEvent struct {
	Log       string
	Processed int
	Total     int
}

type ProgressCallback func(ProgressEvent)

var runtimeCfg Config

func SetConfig(cfg Config) {
	runtimeCfg = cfg
}

func progressCallbackFromParams(p map[string]interface{}) ProgressCallback {
	if p == nil {
		return nil
	}
	cb, _ := p["__progress_cb"].(ProgressCallback)
	return cb
}

func emitProgress(p map[string]interface{}, log string, processed, total int) {
	cb := progressCallbackFromParams(p)
	if cb == nil {
		return
	}
	defer func() {
		_ = recover()
	}()
	cb(ProgressEvent{
		Log:       strings.TrimSpace(log),
		Processed: processed,
		Total:     total,
	})
}

func normalizeBaseURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return s
	}
	return strings.TrimRight(s, "/")
}

func joinBaseURL(base, path string) string {
	base = normalizeBaseURL(base)
	if base == "" {
		return "/" + strings.TrimLeft(path, "/")
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func adEndpoint(path string) string {
	return joinBaseURL(runtimeCfg.ADAPIURL, path)
}

func printEndpoint(path string) string {
	return joinBaseURL(runtimeCfg.PrintAPIURL, path)
}

func Login(projectType, username, password string) (projectResult, error) {
	switch projectType {
	case "ad":
		if _, err := adLogin(username, password); err != nil {
			return projectResult{OK: false, Message: "AD 登录失败", Error: err.Error()}, nil
		}
		return projectResult{OK: true, Message: "AD 登录成功"}, nil
	case "print":
		if _, err := printLogin(username, password); err != nil {
			return projectResult{OK: false, Message: "打印管理登录失败", Error: err.Error()}, nil
		}
		return projectResult{OK: true, Message: "打印管理登录成功"}, nil
	case "vpn":
		cli, err := vpnLogin(username, password, runtimeCfg.VPNSshAddr, 22)
		if err != nil {
			return projectResult{OK: false, Message: "VPN 登录失败", Error: err.Error()}, nil
		}
		_ = cli.Close()
		return projectResult{OK: true, Message: "VPN 登录成功"}, nil
	default:
		return projectResult{}, fmt.Errorf("unknown project type: %s", projectType)
	}
}

func Operate(projectType, username, password, action string, params map[string]interface{}) (projectResult, error) {
	if params == nil {
		params = map[string]interface{}{}
	}
	switch projectType {
	case "ad":
		cli, err := adLogin(username, password)
		if err != nil {
			return projectResult{}, err
		}
		return adOperate(cli, action, params), nil
	case "print":
		ctx, err := printLogin(username, password)
		if err != nil {
			return projectResult{}, err
		}
		return printOperate(ctx, action, params), nil
	case "vpn":
		params["__vpn_account"] = username
		params["__vpn_password"] = password
		cli, err := vpnLogin(username, password, runtimeCfg.VPNSshAddr, 22)
		if err != nil {
			return projectResult{}, err
		}
		defer cli.Close()
		return vpnOperate(cli, action, params), nil
	default:
		return projectResult{}, fmt.Errorf("unknown project type: %s", projectType)
	}
}

func BatchExcelFiles() ([]string, error) {
	return adBatchExcelFiles()
}

func BatchUploadDir() string {
	return adBatchUploadDir()
}

func BatchTemplatePath() string {
	return adBatchTemplatePath()
}

func newHTTPClient(timeout time.Duration) *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Timeout: timeout, Jar: jar}
}

func postForm(client *http.Client, endpoint string, form url.Values) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return client.Do(req)
}

func decodeRespJSON(resp *http.Response) (map[string]interface{}, error) {
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err = json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("json decode failed: %w, body=%s", err, truncate(string(b), 200))
	}
	return out, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func randomPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	mrand.Seed(time.Now().UnixNano())
	out := []byte{chars[mrand.Intn(26)], chars[26+mrand.Intn(26)], chars[52+mrand.Intn(10)]}
	for i := 0; i < 5; i++ {
		out = append(out, chars[mrand.Intn(len(chars))])
	}
	mrand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return string(out)
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
var strongPwdUpperRegex = regexp.MustCompile(`[A-Z]`)
var strongPwdLowerRegex = regexp.MustCompile(`[a-z]`)
var strongPwdDigitRegex = regexp.MustCompile(`[0-9]`)

func isValidEmail(email string) bool {
	return emailRegex.MatchString(strings.TrimSpace(email))
}

func isValidStrongPassword(password string) bool {
	pwd := strings.TrimSpace(password)
	if len(pwd) < 8 {
		return false
	}
	return strongPwdUpperRegex.MatchString(pwd) &&
		strongPwdLowerRegex.MatchString(pwd) &&
		strongPwdDigitRegex.MatchString(pwd)
}

func normalizeUsers(v interface{}) []string {
	res := make([]string, 0)
	parseOne := func(text string) []string {
		clean := strings.TrimSpace(text)
		if clean == "" {
			return nil
		}
		out := make([]string, 0)
		for _, s := range strings.FieldsFunc(clean, func(r rune) bool { return r == ',' || r == ';' || r == '/' }) {
			t := strings.TrimSpace(s)
			if at := strings.Index(t, "@"); at >= 0 {
				t = strings.TrimSpace(t[:at])
			}
			if t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	switch vv := v.(type) {
	case string:
		res = append(res, parseOne(vv)...)
	case []interface{}:
		for _, one := range vv {
			res = append(res, parseOne(toString(one))...)
		}
	}
	return res
}

func toInt(v interface{}) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(n))
		return i
	default:
		return 0
	}
}

func toString(v interface{}) string {
	switch s := v.(type) {
	case string:
		return s
	case float64:
		if float64(int64(s)) == s {
			return strconv.FormatInt(int64(s), 10)
		}
		return strconv.FormatFloat(s, 'f', -1, 64)
	case bool:
		if s {
			return "true"
		}
		return "false"
	default:
		if v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}
}

func toStringDefault(v interface{}, def string) string {
	s := strings.TrimSpace(toString(v))
	if s == "" {
		return def
	}
	return s
}

func toBool(v interface{}) bool {
	switch b := v.(type) {
	case bool:
		return b
	case float64:
		return b != 0
	case int:
		return b != 0
	case string:
		t := strings.ToLower(strings.TrimSpace(b))
		return t == "1" || t == "true" || t == "yes"
	default:
		return false
	}
}

func toBoolDefault(v interface{}, def bool) bool {
	if v == nil {
		return def
	}
	return toBool(v)
}

func toSlice(v interface{}) []interface{} {
	if v == nil {
		return []interface{}{}
	}
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return []interface{}{}
}

func minInt(nums ...int) int {
	if len(nums) == 0 {
		return 0
	}
	m := nums[0]
	for _, n := range nums[1:] {
		if n < m {
			m = n
		}
	}
	return m
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func init() {
	if tr, ok := http.DefaultTransport.(*http.Transport); ok {
		cloned := tr.Clone()
		cloned.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		http.DefaultTransport = cloned
	}
}

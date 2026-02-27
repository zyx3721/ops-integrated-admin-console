package runtime

import (
	crand "crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

func validProjectType(t string) bool {
	switch t {
	case "ad", "print", "vpn":
		return true
	default:
		return false
	}
}

func validCredentialProjectType(t string) bool {
	switch t {
	case "ad", "print", "vpn", "vpn_firewall":
		return true
	default:
		return false
	}
}

func decodeJSON(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func extractBearerToken(authHeader string) string {
	authHeader = strings.TrimSpace(authHeader)
	if authHeader == "" {
		return ""
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(strings.ToLower(authHeader), strings.ToLower(prefix)) {
		return ""
	}
	return strings.TrimSpace(authHeader[len(prefix):])
}

func randomToken(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := crand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func nowStr() string {
	return time.Now().Format(time.RFC3339)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func normalizeGarbledText(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return raw
	}
	if !looksLikeMojibake(text) {
		return raw
	}
	gbkBytes, err := simplifiedchinese.GB18030.NewEncoder().Bytes([]byte(text))
	if err != nil || !utf8.Valid(gbkBytes) {
		return raw
	}
	fixed := string(gbkBytes)
	if strings.TrimSpace(fixed) == "" {
		return raw
	}
	if strings.ContainsRune(fixed, utf8.RuneError) {
		return raw
	}
	if mojibakeScore(fixed) >= mojibakeScore(text) {
		return raw
	}
	return fixed
}

func looksLikeMojibake(text string) bool {
	return strings.ContainsRune(text, utf8.RuneError)
}

func mojibakeScore(text string) int {
	return strings.Count(text, string(utf8.RuneError))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
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

package runtime

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func loadEnvFiles(paths ...string) {
	for _, p := range paths {
		_ = loadEnvFile(p)
	}
}

func loadEnvFile(path string) error {
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		idx := strings.Index(s, "=")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(s[:idx])
		val := strings.TrimSpace(s[idx+1:])
		val = strings.Trim(val, `"'`)
		if key == "" {
			continue
		}
		if _, ok := os.LookupEnv(key); ok {
			continue
		}
		_ = os.Setenv(key, val)
	}
	return nil
}

func loadAppConfig() appConfig {
	ttlMinutes := envInt("PROJECT_CACHE_TTL_MINUTES", 10)
	if ttlMinutes <= 0 {
		ttlMinutes = 10
	}
	return appConfig{
		ADAPIURL:        normalizeBaseURL(envString("AD_API_URL", "http://10.22.50.248/")),
		PrintAPIURL:     normalizeBaseURL(envString("PRINT_API_URL", "http://printhub.sunline.cn/printhub/")),
		VPNSshAddr:      strings.TrimSpace(envString("VPN_SSH_ADDR", "100.100.100.7")),
		FirewallSSHAddr: strings.TrimSpace(envString("FIREWALL_SSH_ADDR", "100.100.100.2")),
		CredentialKey:   envString("CREDENTIAL_SECRET", "change-me-ops-credential-secret"),
		ProjectCacheTTL: time.Duration(ttlMinutes) * time.Minute,
	}
}

func envString(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
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

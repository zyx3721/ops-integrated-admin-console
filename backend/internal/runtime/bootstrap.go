package runtime

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"ops-admin-backend/internal/project"

	_ "modernc.org/sqlite"
)

type contextKey string

const userKey contextKey = "authed_user"

type authedUser struct {
	ID       int64
	Username string
	Token    string
}

type appConfig struct {
	ADAPIURL        string
	PrintAPIURL     string
	VPNSshAddr      string
	FirewallSSHAddr string
	CredentialKey   string
	ProjectCacheTTL time.Duration
	SessionIdleTTL  time.Duration
}

type server struct {
	db                      *sql.DB
	tokenTTL                time.Duration
	cfg                     appConfig
	jobMu                   sync.Mutex
	jobs                    map[string]*asyncOperateJob
	projectSessions         *projectSessionManager
	browserCloseLogMu       sync.Mutex
	pendingBrowserCloseLogs map[string]*pendingBrowserCloseLog
}

type apiError struct {
	Error string `json:"error"`
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResp struct {
	Token                string `json:"token"`
	Username             string `json:"username"`
	ExpireAt             string `json:"expire_at"`
	DefaultPwd           bool   `json:"default_pwd"`
	ProjectCacheTTLInSec int    `json:"project_cache_ttl_seconds"`
	SessionIdleTTLInSec  int    `json:"session_idle_ttl_seconds"`
}

type changePasswordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type browserCloseEventReq struct {
	Reason         string `json:"reason"`
	ClosedAtMS     int64  `json:"closed_at_ms"`
	TimeoutAtMS    int64  `json:"timeout_at_ms"`
	ReopenedAtMS   int64  `json:"reopened_at_ms"`
	IdleTTLSeconds int    `json:"idle_ttl_seconds"`
}

type projectCredentialReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

type operateReq struct {
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
}

type projectResult = project.Result

type logRow struct {
	ID          int64  `json:"id"`
	UserID      int64  `json:"user_id"`
	Username    string `json:"username"`
	Action      string `json:"action"`
	ProjectType string `json:"project_type"`
	Detail      string `json:"detail"`
	CreatedAt   string `json:"created_at"`
}

type pendingBrowserCloseLog struct {
	user authedUser
	req  browserCloseEventReq
}

var runtimeCfg appConfig

const credentialCipherPrefix = "enc:v1:"
const browserCloseLogGracePeriod = 3 * time.Second

func Run() {
	loadEnvFiles(".env", "../.env")
	cfg := loadAppConfig()
	runtimeCfg = cfg
	project.SetConfig(project.Config{
		ADAPIURL:        cfg.ADAPIURL,
		PrintAPIURL:     cfg.PrintAPIURL,
		VPNSshAddr:      cfg.VPNSshAddr,
		FirewallSSHAddr: cfg.FirewallSSHAddr,
	})

	dbPath := filepath.Clean("./db/ops_admin.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("prepare sqlite dir failed: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("open sqlite failed: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err = db.Exec(`PRAGMA busy_timeout = 5000`); err != nil {
		log.Printf("set busy_timeout failed: %v", err)
	}

	if err = initDB(db, cfg); err != nil {
		log.Fatalf("init db failed: %v", err)
	}

	srv := &server{
		db:                      db,
		tokenTTL:                24 * time.Hour,
		cfg:                     cfg,
		jobs:                    make(map[string]*asyncOperateJob),
		projectSessions:         newProjectSessionManager(),
		pendingBrowserCloseLogs: make(map[string]*pendingBrowserCloseLog),
	}

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	h := corsMiddleware(http.HandlerFunc(srv.route))
	log.Printf("backend started on %s, sqlite=%s", addr, dbPath)
	if err = http.ListenAndServe(addr, h); err != nil {
		log.Fatalf("listen failed: %v", err)
	}
}

package runtime

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func initDB(db *sql.DB, cfg appConfig) error {
	schema := []string{
		`CREATE TABLE IF NOT EXISTS admins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS auth_tokens (
			token TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			expires_at TEXT NOT NULL,
			created_at TEXT NOT NULL,
			FOREIGN KEY(user_id) REFERENCES admins(id)
		);`,
		`CREATE TABLE IF NOT EXISTS project_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			project_type TEXT NOT NULL,
			account TEXT NOT NULL DEFAULT '',
			password TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL,
			UNIQUE(user_id, project_type),
			FOREIGN KEY(user_id) REFERENCES admins(id)
		);`,
		`CREATE TABLE IF NOT EXISTS project_load_state (
			user_id INTEGER NOT NULL,
			project_type TEXT NOT NULL,
			loaded INTEGER NOT NULL DEFAULT 0,
			loaded_at TEXT NOT NULL,
			PRIMARY KEY(user_id, project_type),
			FOREIGN KEY(user_id) REFERENCES admins(id)
		);`,
		`CREATE TABLE IF NOT EXISTS operation_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			username TEXT,
			action TEXT NOT NULL,
			project_type TEXT,
			detail TEXT,
			created_at TEXT NOT NULL
		);`,
	}
	for _, stmt := range schema {
		if _, err := db.Exec(stmt); err != nil {
			return err
		}
	}
	var err error

	var cnt int
	if err := db.QueryRow(`SELECT COUNT(1) FROM admins`).Scan(&cnt); err != nil {
		return err
	}
	if cnt == 0 {
		now := nowStr()
		hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err = db.Exec(`INSERT INTO admins(username,password_hash,created_at,updated_at) VALUES(?,?,?,?)`, "admin", string(hash), now, now); err != nil {
			return err
		}
	}

	if err = migrateProjectCredentialsSchema(db); err != nil {
		return err
	}
	if err = ensureDefaultProjectCredentialsForAllUsers(db); err != nil {
		return err
	}
	if err = encryptLegacyProjectCredentialPasswords(db, cfg.CredentialKey); err != nil {
		return err
	}
	return nil
}

func migrateProjectCredentialsSchema(db *sql.DB) error {
	hasUserID, err := tableHasColumn(db, "project_credentials", "user_id")
	if err != nil {
		return err
	}
	if hasUserID {
		return nil
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`CREATE TABLE IF NOT EXISTS project_credentials_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		project_type TEXT NOT NULL,
		account TEXT NOT NULL DEFAULT '',
		password TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL,
		UNIQUE(user_id, project_type),
		FOREIGN KEY(user_id) REFERENCES admins(id)
	);`); err != nil {
		return err
	}

	var defaultUserID int64
	if err = tx.QueryRow(`SELECT id FROM admins ORDER BY id ASC LIMIT 1`).Scan(&defaultUserID); err != nil {
		return err
	}

	rows, err := tx.Query(`SELECT project_type,account,password,updated_at FROM project_credentials`)
	if err != nil {
		return err
	}

	for rows.Next() {
		var projectType, account, password, updatedAt string
		if err = rows.Scan(&projectType, &account, &password, &updatedAt); err != nil {
			_ = rows.Close()
			return err
		}
		if strings.TrimSpace(updatedAt) == "" {
			updatedAt = nowStr()
		}
		if _, err = tx.Exec(`INSERT OR IGNORE INTO project_credentials_new(user_id,project_type,account,password,updated_at) VALUES(?,?,?,?,?)`,
			defaultUserID, projectType, account, password, updatedAt,
		); err != nil {
			return err
		}
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}

	if _, err = tx.Exec(`DROP TABLE project_credentials`); err != nil {
		return err
	}
	if _, err = tx.Exec(`ALTER TABLE project_credentials_new RENAME TO project_credentials`); err != nil {
		return err
	}
	return tx.Commit()
}

func tableHasColumn(db *sql.DB, tableName, columnName string) (bool, error) {
	rows, err := db.Query(fmt.Sprintf(`PRAGMA table_info(%s)`, tableName))
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dfltValue interface{}
		if err = rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if strings.EqualFold(name, columnName) {
			return true, nil
		}
	}
	return false, rows.Err()
}

func ensureDefaultProjectCredentialsForAllUsers(db *sql.DB) error {
	rows, err := db.Query(`SELECT id FROM admins`)
	if err != nil {
		return err
	}
	userIDs := make([]int64, 0)
	for rows.Next() {
		var userID int64
		if err = rows.Scan(&userID); err != nil {
			_ = rows.Close()
			return err
		}
		userIDs = append(userIDs, userID)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, userID := range userIDs {
		if err = ensureDefaultProjectCredentialsForUser(db, userID); err != nil {
			return err
		}
	}
	return nil
}

func ensureDefaultProjectCredentialsForUser(db *sql.DB, userID int64) error {
	for _, p := range []string{"ad", "print", "vpn", "vpn_firewall"} {
		if _, err := db.Exec(`INSERT OR IGNORE INTO project_credentials(user_id,project_type,account,password,updated_at) VALUES(?,?,?,?,?)`, userID, p, "", "", nowStr()); err != nil {
			return err
		}
	}
	return nil
}

func encryptLegacyProjectCredentialPasswords(db *sql.DB, key string) error {
	rows, err := db.Query(`SELECT rowid,password FROM project_credentials`)
	if err != nil {
		return err
	}

	type credentialRow struct {
		ID       int64
		Password string
	}
	all := make([]credentialRow, 0)
	for rows.Next() {
		var item credentialRow
		if err = rows.Scan(&item.ID, &item.Password); err != nil {
			_ = rows.Close()
			return err
		}
		all = append(all, item)
	}
	if err = rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}

	for _, item := range all {
		pwd := strings.TrimSpace(item.Password)
		if pwd == "" || strings.HasPrefix(pwd, credentialCipherPrefix) {
			continue
		}
		encrypted, encErr := encryptCredentialPassword(pwd, key)
		if encErr != nil {
			return encErr
		}
		if _, err = db.Exec(`UPDATE project_credentials SET password=?,updated_at=? WHERE rowid=?`, encrypted, nowStr(), item.ID); err != nil {
			return err
		}
	}
	return nil
}

func encryptCredentialPassword(plain, key string) (string, error) {
	value := strings.TrimSpace(plain)
	if value == "" {
		return "", nil
	}
	if strings.HasPrefix(value, credentialCipherPrefix) {
		return value, nil
	}

	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = crand.Read(nonce); err != nil {
		return "", err
	}
	cipherText := gcm.Seal(nil, nonce, []byte(value), nil)
	raw := append(nonce, cipherText...)
	return credentialCipherPrefix + base64.RawStdEncoding.EncodeToString(raw), nil
}

func credentialCandidateKeys(primary string) []string {
	items := make([]string, 0, 6)
	seen := map[string]bool{}
	appendKey := func(v string) {
		k := strings.TrimSpace(v)
		if k == "" || seen[k] {
			return
		}
		seen[k] = true
		items = append(items, k)
	}

	appendKey(primary)
	for _, one := range strings.Split(envString("CREDENTIAL_SECRET_FALLBACKS", ""), ",") {
		appendKey(one)
	}
	// Built-in historical defaults for smoother key migration.
	appendKey("change-me-ops-credential-secret")
	appendKey("change-this-to-your-own-secret-key")
	return items
}

func decryptCredentialPasswordRaw(raw []byte, key string) (string, error) {
	sum := sha256.Sum256([]byte(key))
	block, err := aes.NewCipher(sum[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	ns := gcm.NonceSize()
	if len(raw) < ns {
		return "", errors.New("invalid credential cipher text")
	}
	nonce, body := raw[:ns], raw[ns:]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func decryptCredentialPassword(cipherText, key string) (string, error) {
	value := strings.TrimSpace(cipherText)
	if value == "" {
		return "", nil
	}
	if !strings.HasPrefix(value, credentialCipherPrefix) {
		return value, nil
	}
	encoded := strings.TrimPrefix(value, credentialCipherPrefix)
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	var lastErr error
	for _, candidate := range credentialCandidateKeys(key) {
		plain, decErr := decryptCredentialPasswordRaw(raw, candidate)
		if decErr == nil {
			return plain, nil
		}
		lastErr = decErr
	}
	if lastErr == nil {
		lastErr = errors.New("credential decrypt failed")
	}
	return "", lastErr
}

package runtime

import (
	"database/sql"
	"errors"
	"strings"
)

func (s *server) cleanupAuthToken(token string, userID int64) {
	token = strings.TrimSpace(token)
	if token == "" {
		return
	}
	s.projectSessions.clearToken(token)
	_, _ = s.db.Exec(`DELETE FROM auth_tokens WHERE token=?`, token)
	_ = userID
}

func (s *server) cleanupUserAuthTokens(userID int64) {
	if userID <= 0 {
		return
	}

	rows, err := s.db.Query(`SELECT token FROM auth_tokens WHERE user_id=?`, userID)
	if err == nil {
		tokens := make([]string, 0)
		for rows.Next() {
			var token string
			if scanErr := rows.Scan(&token); scanErr == nil {
				token = strings.TrimSpace(token)
				if token != "" {
					tokens = append(tokens, token)
				}
			}
		}
		_ = rows.Close()
		for _, token := range tokens {
			s.projectSessions.clearToken(token)
		}
	}

	_, _ = s.db.Exec(`DELETE FROM auth_tokens WHERE user_id=?`, userID)
}

func (s *server) loadAuthedUser(token, now string) (authedUser, error) {
	var u authedUser
	err := s.db.QueryRow(
		`SELECT a.id,a.username,t.token FROM auth_tokens t JOIN admins a ON a.id=t.user_id WHERE t.token=? AND t.expires_at>?`,
		token,
		now,
	).Scan(&u.ID, &u.Username, &u.Token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return authedUser{}, sql.ErrNoRows
		}
		return authedUser{}, err
	}
	return u, nil
}

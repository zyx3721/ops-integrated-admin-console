package runtime

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"ops-admin-backend/internal/project"
)

type managedProjectSession struct {
	token       string
	userID      int64
	projectType string
	username    string
	password    string
	session     project.Session
	loadedAt    time.Time
	lastUsedAt  time.Time
}

type projectSessionManager struct {
	mu       sync.Mutex
	sessions map[string]map[string]*managedProjectSession
}

func newProjectSessionManager() *projectSessionManager {
	return &projectSessionManager{
		sessions: make(map[string]map[string]*managedProjectSession),
	}
}

func (m *projectSessionManager) ensure(u authedUser, projectType, username, password string, ttl time.Duration, forceRelogin bool) (*managedProjectSession, bool, string, error) {
	now := time.Now()

	m.mu.Lock()
	existing := m.getLocked(u.Token, projectType)
	if existing != nil && !forceRelogin && !m.isExpiredLocked(existing, username, password, ttl, now) {
		existing.lastUsedAt = now
		m.mu.Unlock()
		return existing, false, "", nil
	}
	if existing != nil {
		m.removeLocked(u.Token, projectType)
	}
	m.mu.Unlock()

	session, message, err := project.OpenSession(projectType, username, password)
	if err != nil {
		if message == "" {
			message = loginFailureMessage(projectType)
		}
		return nil, false, message, err
	}
	if session == nil {
		return nil, false, "", fmt.Errorf("unknown project type: %s", projectType)
	}

	entry := &managedProjectSession{
		token:       u.Token,
		userID:      u.ID,
		projectType: projectType,
		username:    username,
		password:    password,
		session:     session,
		loadedAt:    now,
		lastUsedAt:  now,
	}

	var stale project.Session
	m.mu.Lock()
	current := m.getLocked(u.Token, projectType)
	if current != nil {
		stale = current.session
	}
	m.setLocked(entry)
	m.mu.Unlock()

	if stale != nil && stale != session {
		_ = stale.Close()
	}
	return entry, true, message, nil
}

func (m *projectSessionManager) get(token, projectType string) *managedProjectSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getLocked(token, projectType)
}

func (m *projectSessionManager) clearToken(token string) {
	if strings.TrimSpace(token) == "" {
		return
	}
	toClose := make([]project.Session, 0)
	m.mu.Lock()
	if items, ok := m.sessions[token]; ok {
		for _, item := range items {
			if item != nil && item.session != nil {
				toClose = append(toClose, item.session)
			}
		}
		delete(m.sessions, token)
	}
	m.mu.Unlock()
	for _, one := range toClose {
		_ = one.Close()
	}
}

func (m *projectSessionManager) clearTokenProject(token, projectType string) {
	m.clearMatching(func(item *managedProjectSession) bool {
		return item.token == token && item.projectType == projectType
	})
}

func (m *projectSessionManager) clearUserProject(userID int64, projectType string) {
	m.clearMatching(func(item *managedProjectSession) bool {
		return item.userID == userID && item.projectType == projectType
	})
}

func (m *projectSessionManager) clearMatching(match func(*managedProjectSession) bool) {
	toClose := make([]project.Session, 0)
	m.mu.Lock()
	for token, items := range m.sessions {
		for projectType, item := range items {
			if item == nil || !match(item) {
				continue
			}
			if item.session != nil {
				toClose = append(toClose, item.session)
			}
			delete(items, projectType)
		}
		if len(items) == 0 {
			delete(m.sessions, token)
		}
	}
	m.mu.Unlock()
	for _, one := range toClose {
		_ = one.Close()
	}
}

func (m *projectSessionManager) getLocked(token, projectType string) *managedProjectSession {
	items := m.sessions[token]
	if items == nil {
		return nil
	}
	return items[projectType]
}

func (m *projectSessionManager) setLocked(entry *managedProjectSession) {
	items := m.sessions[entry.token]
	if items == nil {
		items = make(map[string]*managedProjectSession)
		m.sessions[entry.token] = items
	}
	items[entry.projectType] = entry
}

func (m *projectSessionManager) removeLocked(token, projectType string) {
	items := m.sessions[token]
	if items == nil {
		return
	}
	item := items[projectType]
	delete(items, projectType)
	if len(items) == 0 {
		delete(m.sessions, token)
	}
	if item != nil && item.session != nil {
		go func(session project.Session) {
			_ = session.Close()
		}(item.session)
	}
}

func (m *projectSessionManager) isExpiredLocked(item *managedProjectSession, username, password string, ttl time.Duration, now time.Time) bool {
	if item == nil || item.session == nil {
		return true
	}
	if item.username != username || item.password != password {
		return true
	}
	if ttl > 0 && now.Sub(item.loadedAt) >= ttl {
		return true
	}
	return false
}

func loginFailureMessage(projectType string) string {
	switch projectType {
	case "ad":
		return "AD 登录失败"
	case "print":
		return "打印管理登录失败"
	case "vpn":
		return "VPN 登录失败"
	default:
		return "项目登录失败"
	}
}

func (s *server) ensureProjectSession(u authedUser, projectType string, forceRelogin bool) (*managedProjectSession, bool, string, error) {
	account, password, err := s.getProjectCredential(u.ID, projectType)
	if err != nil {
		return nil, false, "", err
	}
	return s.projectSessions.ensure(u, projectType, account, password, s.cfg.ProjectCacheTTL, forceRelogin)
}

func (s *server) operateWithProjectSession(entry *managedProjectSession, action string, params map[string]interface{}) (projectResult, error) {
	if entry == nil || entry.session == nil {
		return projectResult{}, errors.New("project session not initialized")
	}
	if params == nil {
		params = map[string]interface{}{}
	}
	if entry.projectType == "vpn" {
		params["__vpn_account"] = entry.username
		params["__vpn_password"] = entry.password
	}
	entry.lastUsedAt = time.Now()
	return entry.session.Operate(action, params)
}

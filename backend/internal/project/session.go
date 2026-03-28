package project

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

type Session interface {
	Operate(action string, params map[string]interface{}) (projectResult, error)
	Close() error
}

type adSession struct {
	client *http.Client
}

func (s *adSession) Operate(action string, params map[string]interface{}) (projectResult, error) {
	return adOperate(s.client, action, params), nil
}

func (s *adSession) Close() error {
	return nil
}

type printSession struct {
	ctx *printCtx
}

func (s *printSession) Operate(action string, params map[string]interface{}) (projectResult, error) {
	return printOperate(s.ctx, action, params), nil
}

func (s *printSession) Close() error {
	return nil
}

type vpnSession struct {
	mu       sync.Mutex
	username string
	password string
	host     string
	client   *ssh.Client
}

func newVPNSession(username, password, host string) (*vpnSession, error) {
	client, err := vpnLogin(username, password, host, 22)
	if err != nil {
		return nil, err
	}
	return &vpnSession{
		username: username,
		password: password,
		host:     host,
		client:   client,
	}, nil
}

func (s *vpnSession) Operate(action string, params map[string]interface{}) (projectResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureClientLocked(); err != nil {
		return projectResult{}, err
	}

	result := vpnOperate(s.client, action, params)
	if !shouldReconnectVPNResult(result) {
		return result, nil
	}

	if err := s.reconnectLocked(); err != nil {
		return result, nil
	}
	return vpnOperate(s.client, action, params), nil
}

func (s *vpnSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return nil
	}
	err := s.client.Close()
	s.client = nil
	return err
}

func (s *vpnSession) ensureClientLocked() error {
	if s.client != nil {
		return nil
	}
	return s.reconnectLocked()
}

func (s *vpnSession) reconnectLocked() error {
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	client, err := vpnLogin(s.username, s.password, s.host, 22)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

func shouldReconnectVPNResult(res projectResult) bool {
	if res.OK {
		return false
	}
	text := strings.ToLower(strings.TrimSpace(res.Error))
	if text == "" {
		text = strings.ToLower(strings.TrimSpace(res.Message))
	}
	if text == "" {
		return false
	}
	for _, one := range []string{
		"connection reset",
		"broken pipe",
		"closed network connection",
		"use of closed network connection",
		"failed to open session",
		"unable to open channel",
		"eof",
	} {
		if strings.Contains(text, one) {
			return true
		}
	}
	return false
}

func OpenSession(projectType, username, password string) (Session, string, error) {
	switch projectType {
	case "ad":
		client, err := adLogin(username, password)
		if err != nil {
			return nil, "AD 登录失败", err
		}
		return &adSession{client: client}, "AD 登录成功", nil
	case "print":
		ctx, err := printLogin(username, password)
		if err != nil {
			return nil, "打印管理登录失败", err
		}
		return &printSession{ctx: ctx}, "打印管理登录成功", nil
	case "vpn":
		session, err := newVPNSession(username, password, runtimeCfg.VPNSshAddr)
		if err != nil {
			return nil, "VPN 登录失败", err
		}
		return session, "VPN 登录成功", nil
	default:
		return nil, "", fmt.Errorf("unknown project type: %s", projectType)
	}
}

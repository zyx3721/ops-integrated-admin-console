package runtime

import "ops-admin-backend/internal/project"

func (s *server) projectLogin(projectType, username, password string) (projectResult, error) {
	return project.Login(projectType, username, password)
}

func (s *server) projectOperate(projectType, username, password, action string, params map[string]interface{}) (projectResult, error) {
	return project.Operate(projectType, username, password, action, params)
}

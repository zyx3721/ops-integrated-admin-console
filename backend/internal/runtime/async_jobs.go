package runtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"ops-admin-backend/internal/project"
)

const (
	asyncJobStatusRunning = "running"
	asyncJobStatusSuccess = "success"
	asyncJobStatusFailed  = "failed"
)

type asyncOperateReq struct {
	ProjectType string                 `json:"project_type"`
	Action      string                 `json:"action"`
	Params      map[string]interface{} `json:"params"`
}

type asyncOperateJob struct {
	ID          string
	UserID      int64
	Username    string
	ProjectType string
	Action      string
	Status      string
	OK          bool
	Done        bool
	Message     string
	Error       string
	Progress    int
	Processed   int
	Total       int
	LogLines    []string
	ResultText  string
	ResultItems []interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type asyncOperateJobView struct {
	JobID       string        `json:"job_id"`
	ProjectType string        `json:"project_type"`
	Action      string        `json:"action"`
	Status      string        `json:"status"`
	OK          bool          `json:"ok"`
	Done        bool          `json:"done"`
	Message     string        `json:"message"`
	Error       string        `json:"error"`
	Progress    int           `json:"progress"`
	Processed   int           `json:"processed"`
	Total       int           `json:"total"`
	LogLines    []string      `json:"log_lines"`
	ResultText  string        `json:"result_text"`
	ResultItems []interface{} `json:"result_items"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
}

func (s *server) handleProjectOperateAsyncStart(w http.ResponseWriter, r *http.Request, u authedUser) {
	var req asyncOperateReq
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "请求体格式错误"})
		return
	}
	req.ProjectType = strings.TrimSpace(req.ProjectType)
	req.Action = strings.TrimSpace(req.Action)
	if !validProjectType(req.ProjectType) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "无效的项目类型"})
		return
	}
	if req.Action == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "操作类型不能为空"})
		return
	}
	params := cloneInterfaceMap(req.Params)
	if params == nil {
		params = map[string]interface{}{}
	}

	if req.ProjectType == "vpn" && req.Action == "delete_users" && toBoolDefault(params["remote_firewall"], false) {
		fwAccount, fwPassword, fwErr := s.getProjectCredential(u.ID, "vpn_firewall")
		if fwErr != nil {
			params["__vpn_fw_configured"] = false
			params["__vpn_fw_error"] = fwErr.Error()
		} else {
			params["__vpn_fw_configured"] = true
			params["__vpn_fw_account"] = fwAccount
			params["__vpn_fw_password"] = fwPassword
		}
	}

	account, password, err := s.getProjectCredential(u.ID, req.ProjectType)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: err.Error()})
		return
	}

	job, err := s.createAsyncOperateJob(u, req.ProjectType, req.Action)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "创建异步任务失败"})
		return
	}
	go s.runAsyncOperate(job.ID, u, req.ProjectType, req.Action, params, account, password)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"job_id":       job.ID,
		"status":       job.Status,
		"created_at":   job.CreatedAt.Format(time.RFC3339),
		"project_type": req.ProjectType,
		"action":       req.Action,
	})
}

func (s *server) handleProjectOperateAsyncStatus(w http.ResponseWriter, r *http.Request, u authedUser) {
	jobID := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/api/projects/operate-async/"))
	if jobID == "" || strings.Contains(jobID, "/") {
		writeJSON(w, http.StatusNotFound, apiError{Error: "任务不存在"})
		return
	}
	view, ok := s.getAsyncOperateJobView(jobID, u.ID)
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "任务不存在或已过期"})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *server) createAsyncOperateJob(u authedUser, projectType, action string) (*asyncOperateJob, error) {
	id, err := randomToken(18)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	job := &asyncOperateJob{
		ID:          id,
		UserID:      u.ID,
		Username:    u.Username,
		ProjectType: projectType,
		Action:      action,
		Status:      asyncJobStatusRunning,
		OK:          false,
		Done:        false,
		Progress:    1,
		Processed:   0,
		Total:       0,
		LogLines:    []string{"开始执行..."},
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	if s.jobs == nil {
		s.jobs = make(map[string]*asyncOperateJob)
	}
	s.purgeAsyncJobsLocked(now)
	s.jobs[job.ID] = job
	return job, nil
}

func (s *server) runAsyncOperate(jobID string, u authedUser, projectType, action string, params map[string]interface{}, account, password string) {
	progressCB := project.ProgressCallback(func(ev project.ProgressEvent) {
		s.updateAsyncOperateJob(jobID, func(job *asyncOperateJob) {
			line := strings.TrimSpace(ev.Log)
			if line != "" {
				job.LogLines = append(job.LogLines, line)
				if len(job.LogLines) > 2000 {
					job.LogLines = job.LogLines[len(job.LogLines)-2000:]
				}
				job.ResultText = strings.Join(job.LogLines, "\n")
			}
			if ev.Total > 0 {
				job.Total = ev.Total
			}
			if ev.Processed > 0 {
				job.Processed = ev.Processed
			}
			job.Progress = calcJobProgress(job.Processed, job.Total, len(job.LogLines), false)
		})
	})
	if params == nil {
		params = map[string]interface{}{}
	}
	params["__progress_cb"] = progressCB

	res, err := s.projectOperate(projectType, account, password, action, params)
	if err != nil {
		errMsg := strings.TrimSpace(err.Error())
		if errMsg == "" {
			errMsg = "执行失败"
		}
		s.updateAsyncOperateJob(jobID, func(job *asyncOperateJob) {
			job.Status = asyncJobStatusFailed
			job.OK = false
			job.Done = true
			job.Message = "执行失败"
			job.Error = errMsg
			job.Progress = 100
			job.LogLines = append(job.LogLines, "执行失败："+errMsg)
			job.ResultText = strings.Join(job.LogLines, "\n")
		})
		s.logAction(u.ID, u.Username, "project_operate_failed", projectType, fmt.Sprintf("action=%s, err=%s", action, errMsg))
		return
	}

	if !res.OK {
		errMsg := strings.TrimSpace(res.Error)
		if errMsg == "" {
			errMsg = strings.TrimSpace(res.Message)
		}
		if errMsg == "" {
			errMsg = "执行失败"
		}
		s.updateAsyncOperateJob(jobID, func(job *asyncOperateJob) {
			job.Status = asyncJobStatusFailed
			job.OK = false
			job.Done = true
			job.Message = strings.TrimSpace(res.Message)
			if job.Message == "" {
				job.Message = "执行失败"
			}
			job.Error = errMsg
			job.Progress = 100
			job.LogLines = append(job.LogLines, "执行失败："+errMsg)
			if logText := extractLogText(res.Data); logText != "" {
				job.ResultText = logText
			} else {
				job.ResultText = strings.Join(job.LogLines, "\n")
			}
			job.ResultItems = normalizeResultItems(res.Data)
		})
		s.logAction(u.ID, u.Username, "project_operate_failed", projectType, fmt.Sprintf("action=%s, err=%s", action, errMsg))
		return
	}

	s.updateAsyncOperateJob(jobID, func(job *asyncOperateJob) {
		job.Status = asyncJobStatusSuccess
		job.OK = true
		job.Done = true
		job.Message = strings.TrimSpace(res.Message)
		if job.Message == "" {
			job.Message = "执行成功"
		}
		job.Error = ""
		job.ResultItems = normalizeResultItems(res.Data)
		logText := extractLogText(res.Data)
		if logText != "" {
			if len(job.LogLines) <= 1 {
				job.LogLines = splitLogTextLines(logText)
			}
			job.ResultText = logText
		} else if len(job.LogLines) > 0 {
			job.ResultText = strings.Join(job.LogLines, "\n")
		} else {
			job.ResultText = marshalResultAsText(res)
		}
		if job.Processed <= 0 && len(job.ResultItems) > 0 {
			job.Processed = len(job.ResultItems)
		}
		if job.Total <= 0 && job.Processed > 0 {
			job.Total = job.Processed
		}
		job.Progress = 100
	})
	s.logAction(u.ID, u.Username, "project_operate", projectType, fmt.Sprintf("action=%s", action))
}

func calcJobProgress(processed, total, logCount int, done bool) int {
	if done {
		return 100
	}
	if total > 0 && processed >= 0 {
		pct := int(float64(processed) / float64(total) * 100)
		if pct < 1 {
			pct = 1
		}
		if pct > 99 {
			pct = 99
		}
		return pct
	}
	if logCount <= 0 {
		return 1
	}
	pct := 10 + logCount*5
	if pct > 95 {
		pct = 95
	}
	return pct
}

func (s *server) updateAsyncOperateJob(jobID string, fn func(*asyncOperateJob)) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok {
		return
	}
	fn(job)
	job.UpdatedAt = time.Now()
}

func (s *server) getAsyncOperateJobView(jobID string, userID int64) (asyncOperateJobView, bool) {
	s.jobMu.Lock()
	defer s.jobMu.Unlock()
	job, ok := s.jobs[jobID]
	if !ok || job.UserID != userID {
		return asyncOperateJobView{}, false
	}
	view := asyncOperateJobView{
		JobID:       job.ID,
		ProjectType: job.ProjectType,
		Action:      job.Action,
		Status:      job.Status,
		OK:          job.OK,
		Done:        job.Done,
		Message:     job.Message,
		Error:       job.Error,
		Progress:    job.Progress,
		Processed:   job.Processed,
		Total:       job.Total,
		LogLines:    append([]string(nil), job.LogLines...),
		ResultText:  job.ResultText,
		ResultItems: append([]interface{}(nil), job.ResultItems...),
		CreatedAt:   job.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   job.UpdatedAt.Format(time.RFC3339),
	}
	return view, true
}

func (s *server) purgeAsyncJobsLocked(now time.Time) {
	if s.jobs == nil {
		return
	}
	const keepDuration = 30 * time.Minute
	for id, job := range s.jobs {
		if job == nil {
			delete(s.jobs, id)
			continue
		}
		if job.Done && now.Sub(job.UpdatedAt) > keepDuration {
			delete(s.jobs, id)
		}
	}

	if len(s.jobs) <= 400 {
		return
	}
	ids := make([]string, 0, len(s.jobs))
	for id := range s.jobs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		ai := s.jobs[ids[i]]
		aj := s.jobs[ids[j]]
		if ai == nil || aj == nil {
			return ids[i] < ids[j]
		}
		return ai.UpdatedAt.Before(aj.UpdatedAt)
	})
	for _, id := range ids {
		if len(s.jobs) <= 300 {
			break
		}
		delete(s.jobs, id)
	}
}

func cloneInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		out := make(map[string]interface{}, len(in))
		for k, v := range in {
			out[k] = v
		}
		return out
	}
	var out map[string]interface{}
	if err = json.Unmarshal(b, &out); err != nil {
		out = make(map[string]interface{}, len(in))
		for k, v := range in {
			out[k] = v
		}
	}
	return out
}

func extractLogText(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(data["log_text"]))
	if text == "" || text == "<nil>" {
		return ""
	}
	return normalizeGarbledText(text)
}

func normalizeResultItems(data map[string]interface{}) []interface{} {
	if data == nil {
		return []interface{}{}
	}
	raw := data["items"]
	if raw == nil {
		return []interface{}{}
	}
	if out, ok := raw.([]interface{}); ok {
		return out
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return []interface{}{}
	}
	var out []interface{}
	if err = json.Unmarshal(b, &out); err != nil {
		return []interface{}{}
	}
	return out
}

func splitLogTextLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := make([]string, 0)
	for _, one := range strings.Split(text, "\n") {
		line := strings.TrimSpace(one)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 && strings.TrimSpace(text) != "" {
		lines = append(lines, strings.TrimSpace(text))
	}
	return lines
}

func marshalResultAsText(res projectResult) string {
	b, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", res)
	}
	return string(b)
}

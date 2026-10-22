package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// CronJob represents a persisted scheduled task.
type CronJob struct {
	ID          string    `json:"id"`
	Project     string    `json:"project"`
	SessionKey  string    `json:"session_key"`
	CronExpr    string    `json:"cron_expr"`
	Prompt      string    `json:"prompt"`
	Model       string    `json:"model,omitempty"`    // per-run prompt model; empty = inherit project/session defaults
	Exec        string    `json:"exec,omitempty"`     // shell command; mutually exclusive with Prompt
	WorkDir     string    `json:"work_dir,omitempty"` // working directory for exec; empty = agent work_dir
	Description string    `json:"description"`
	Enabled     bool      `json:"enabled"`
	Silent      *bool     `json:"silent,omitempty"`       // suppress start notification; nil = use global default
	Mute        bool      `json:"mute,omitempty"`         // suppress ALL messages (start + result); job runs silently
	SessionMode string    `json:"session_mode,omitempty"` // "" or "reuse" = share active session; "new_per_run" = fresh session each run
	Mode        string    `json:"mode,omitempty"`         // permission mode override for this job; "" = use project default
	TimeoutMins *int      `json:"timeout_mins,omitempty"` // nil = default 30m wait; 0 = no limit; >0 = minutes
	CreatedAt   time.Time `json:"created_at"`
	LastRun     time.Time `json:"last_run,omitempty"`
	LastError   string    `json:"last_error,omitempty"`
}

// IsShellJob returns true if the job runs a shell command directly.
func (j *CronJob) IsShellJob() bool {
	return j.Exec != ""
}

const defaultCronJobTimeout = 30 * time.Minute

var (
	ErrCronJobNotFound     = errors.New("cron job not found")
	ErrCronProjectNotFound = errors.New("cron project not found")
)

// ExecutionTimeout returns how long the scheduler waits for the job goroutine to finish.
// nil TimeoutMins uses 30 minutes. *TimeoutMins == 0 means wait without a time limit.
// *TimeoutMins > 0 means that many minutes.
func (j *CronJob) ExecutionTimeout() time.Duration {
	if j.TimeoutMins == nil {
		return defaultCronJobTimeout
	}
	if *j.TimeoutMins <= 0 {
		return 0
	}
	return time.Duration(*j.TimeoutMins) * time.Minute
}

// UsesNewSessionPerRun reports whether each cron run should use a new engine session
// instead of reusing the active session for the session_key.
func (j *CronJob) UsesNewSessionPerRun() bool {
	return NormalizeCronSessionMode(j.SessionMode) == "new_per_run"
}

// NormalizeCronSessionMode maps CLI/API aliases to canonical values ("", "new_per_run").
// Returns the original string if unrecognized (caller should validate).
func NormalizeCronSessionMode(s string) string {
	s = strings.TrimSpace(s)
	low := strings.ToLower(s)
	switch low {
	case "", "reuse":
		return ""
	case "new_per_run", "new-per-run":
		return "new_per_run"
	default:
		return s
	}
}

func validateCronJob(j *CronJob) error {
	if err := validateCronModel(j); err != nil {
		return err
	}
	// SessionKey anchors the cron execution to a platform (ExecuteCronJob
	// derives platformName from the prefix before ":"). Without it the job
	// is persisted but fails at fire-time with the unhelpful
	// `platform "" not found for session ""`. Reject it up front so the
	// caller (management API, /cron/add, /cron edit) sees an immediate
	// 400 instead of a job that silently never runs.
	if strings.TrimSpace(j.SessionKey) == "" {
		return fmt.Errorf("session_key is required")
	}
	mode := NormalizeCronSessionMode(j.SessionMode)
	if mode != "" && mode != "new_per_run" {
		return fmt.Errorf("invalid session_mode %q (want reuse, new_per_run, or new-per-run)", j.SessionMode)
	}
	if j.Mode != "" {
		switch j.Mode {
		case "default", "bypassPermissions", "acceptEdits", "plan", "auto", "dontAsk":
		default:
			return fmt.Errorf("invalid mode %q (want default, bypassPermissions, acceptEdits, plan, auto, or dontAsk)", j.Mode)
		}
	}
	if j.TimeoutMins != nil && *j.TimeoutMins < 0 {
		return fmt.Errorf("timeout_mins must be >= 0")
	}
	return nil
}

func validateCronModel(j *CronJob) error {
	if strings.TrimSpace(j.Model) != "" && (j.IsShellJob() || strings.TrimSpace(j.Prompt) == "") {
		return fmt.Errorf("model is only supported for prompt jobs")
	}
	return nil
}

// CronStore persists cron jobs to a JSON file.
type CronStore struct {
	path string
	mu   sync.Mutex
	jobs []*CronJob
}

func NewCronStore(dataDir string) (*CronStore, error) {
	dir := filepath.Join(dataDir, "crons")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "jobs.json")
	s := &CronStore{path: path}
	s.load()
	return s, nil
}

func (s *CronStore) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &s.jobs); err != nil {
		slog.Error("cron: failed to load jobs", "path", s.path, "error", err)
	}
}

func (s *CronStore) save() error {
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(s.path, data, 0o644)
}

func (s *CronStore) Add(job *CronJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.jobs = append(s.jobs, job)
	return s.save()
}

func (s *CronStore) Remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, j := range s.jobs {
		if j.ID == id {
			s.jobs = append(s.jobs[:i], s.jobs[i+1:]...)
			if err := s.save(); err != nil {
				slog.Warn("cron: failed to save after remove", "error", err)
			}
			return true
		}
	}
	return false
}

func (s *CronStore) SetEnabled(id string, enabled bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			j.Enabled = enabled
			if err := s.save(); err != nil {
				slog.Warn("cron: failed to save after set enabled", "error", err)
			}
			return true
		}
	}
	return false
}

func (s *CronStore) SetMute(id string, mute bool) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			j.Mute = mute
			if err := s.save(); err != nil {
				slog.Warn("cron: save after mute toggle", "error", err)
			}
			return true
		}
	}
	return false
}

func (s *CronStore) ToggleMute(id string) (newState bool, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			j.Mute = !j.Mute
			if err := s.save(); err != nil {
				slog.Warn("cron: failed to save after toggle mute", "error", err)
			}
			return j.Mute, true
		}
	}
	return false, false
}

func (s *CronStore) MarkRun(id string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			j.LastRun = time.Now()
			if err != nil {
				j.LastError = err.Error()
			} else {
				j.LastError = ""
			}
			if saveErr := s.save(); saveErr != nil {
				slog.Warn("cron: failed to save after mark run", "error", saveErr)
			}
			return
		}
	}
}

func (s *CronStore) List() []*CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*CronJob, len(s.jobs))
	copy(out, s.jobs)
	return out
}

func (s *CronStore) ListByProject(project string) []*CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*CronJob
	for _, j := range s.jobs {
		if j.Project == project {
			out = append(out, j)
		}
	}
	return out
}

func (s *CronStore) ListBySessionKey(sessionKey string) []*CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*CronJob
	for _, j := range s.jobs {
		if j.SessionKey == sessionKey {
			out = append(out, j)
		}
	}
	return out
}

func (s *CronStore) Get(id string) *CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.ID == id {
			return j
		}
	}
	return nil
}

// Snapshot freezes a run's configuration under the store lock. Later edits apply
// to subsequent runs, including edits to pointer-valued settings.
func (s *CronStore) Snapshot(id string) *CronJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, job := range s.jobs {
		if job.ID == id {
			return cloneCronJob(job)
		}
	}
	return nil
}

func cloneCronJob(job *CronJob) *CronJob {
	copy := *job
	if job.Silent != nil {
		value := *job.Silent
		copy.Silent = &value
	}
	if job.TimeoutMins != nil {
		value := *job.TimeoutMins
		copy.TimeoutMins = &value
	}
	return &copy
}

// Update modifies one field, retaining the legacy boolean store API.
func (s *CronStore) Update(id string, field string, value any) bool {
	_, err := s.updateFields(id, map[string]any{field: value}, nil)
	return err == nil
}

// updateFields validates and commits a complete edit under one lock. In
// particular, clearing model while converting a prompt job to exec is atomic.
func (s *CronStore) updateFields(id string, updates map[string]any, validate func(*CronJob) error) (*CronJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, job := range s.jobs {
		if job.ID != id {
			continue
		}
		candidate := cloneCronJob(job)
		for field, value := range updates {
			switch field {
			case "id", "created_at", "last_run", "last_error":
				return nil, fmt.Errorf("field %q is read-only", field)
			}
			if err := updateJobField(candidate, field, value); err != nil {
				return nil, err
			}
		}
		if err := validateCronModel(candidate); err != nil {
			return nil, err
		}
		if validate != nil {
			if err := validate(candidate); err != nil {
				return nil, err
			}
		}
		// Publish a new value instead of rewriting fields on pointers already
		// handed to readers (including the immutable ID used by CLI callers).
		s.jobs[index] = candidate
		if err := s.save(); err != nil {
			s.jobs[index] = job
			return nil, fmt.Errorf("save cron edit: %w", err)
		}
		return cloneCronJob(candidate), nil
	}
	return nil, fmt.Errorf("job %q not found", id)
}

// updateJobField sets a field on a CronJob by reflection. Returns error for unknown fields.
func updateJobField(job *CronJob, field string, value any) error {
	switch field {
	case "project":
		if v, ok := value.(string); ok {
			job.Project = v
			return nil
		}
	case "session_key":
		if v, ok := value.(string); ok {
			job.SessionKey = v
			return nil
		}
	case "cron_expr":
		if v, ok := value.(string); ok {
			job.CronExpr = v
			return nil
		}
	case "prompt":
		if v, ok := value.(string); ok {
			job.Prompt = v
			return nil
		}
	case "model":
		if v, ok := value.(string); ok {
			job.Model = strings.TrimSpace(v)
			return nil
		}
	case "exec":
		if v, ok := value.(string); ok {
			job.Exec = v
			return nil
		}
	case "work_dir":
		if v, ok := value.(string); ok {
			job.WorkDir = v
			return nil
		}
	case "description":
		if v, ok := value.(string); ok {
			job.Description = v
			return nil
		}
	case "enabled":
		if v, ok := value.(bool); ok {
			job.Enabled = v
			return nil
		}
	case "silent":
		if v, ok := value.(bool); ok {
			job.Silent = &v
			return nil
		}
	case "mute":
		if v, ok := value.(bool); ok {
			job.Mute = v
			return nil
		}
	case "session_mode":
		if v, ok := value.(string); ok {
			job.SessionMode = v
			return nil
		}
	case "mode":
		if v, ok := value.(string); ok {
			job.Mode = v
			return nil
		}
	case "timeout_mins":
		if v, ok := value.(float64); ok {
			n := int(v)
			job.TimeoutMins = &n
			return nil
		}
		if v, ok := value.(int); ok {
			job.TimeoutMins = &v
			return nil
		}
	}
	// Fallback: try to set string field via reflection
	if v, ok := value.(string); ok {
		rv := reflect.ValueOf(job).Elem()
		f := rv.FieldByName(toExportedFieldName(field))
		if f.IsValid() && f.Kind() == reflect.String && f.CanSet() {
			f.SetString(v)
			return nil
		}
	}
	return fmt.Errorf("unknown or invalid field: %s", field)
}

// toExportedFieldName converts snake_case to Go exported field name (e.g., "session_key" -> "SessionKey")
func toExportedFieldName(s string) string {
	result := make([]byte, 0, len(s))
	upperNext := true
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '_' {
			upperNext = true
			continue
		}
		if upperNext {
			if c >= 'a' && c <= 'z' {
				c -= 32
			}
			upperNext = false
		}
		result = append(result, c)
	}
	return string(result)
}

// CronScheduler runs cron jobs by injecting synthetic messages into engines.
type CronScheduler struct {
	store              *CronStore
	cron               *cron.Cron
	engines            map[string]*Engine // project name → engine
	mu                 sync.RWMutex
	editMu             sync.Mutex              // serialize multi-field edits with rescheduling
	entries            map[string]cron.EntryID // job ID → cron entry
	defaultSilent      bool                    // global default for suppressing cron start notifications
	defaultSessionMode string                  // global default session mode; "" = reuse, "new_per_run" = fresh session each run
}

func NewCronScheduler(store *CronStore) *CronScheduler {
	return &CronScheduler{
		store:   store,
		cron:    cron.New(),
		engines: make(map[string]*Engine),
		entries: make(map[string]cron.EntryID),
	}
}

func (cs *CronScheduler) RegisterEngine(name string, e *Engine) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.engines[name] = e
}

func (cs *CronScheduler) SetDefaultSilent(silent bool) {
	cs.defaultSilent = silent
}

func (cs *CronScheduler) SetDefaultSessionMode(mode string) {
	cs.defaultSessionMode = NormalizeCronSessionMode(mode)
}

// IsSilent returns whether the cron job should suppress the start notification.
func (cs *CronScheduler) IsSilent(job *CronJob) bool {
	if job.Silent != nil {
		return *job.Silent
	}
	return cs.defaultSilent
}

// UsesNewSession returns whether the job should create a fresh session per run,
// considering both the job-level setting and the global default.
func (cs *CronScheduler) UsesNewSession(job *CronJob) bool {
	if job.SessionMode != "" {
		return job.UsesNewSessionPerRun()
	}
	return cs.defaultSessionMode == "new_per_run"
}

func (cs *CronScheduler) Start() error {
	jobs := cs.store.List()
	for _, job := range jobs {
		if job.Enabled {
			if err := cs.scheduleJob(job); err != nil {
				slog.Warn("cron: failed to schedule job", "id", job.ID, "error", err)
			}
		}
	}
	cs.cron.Start()
	slog.Info("cron: scheduler started", "jobs", len(jobs))
	return nil
}

func (cs *CronScheduler) Stop() {
	cs.cron.Stop()
}

func (cs *CronScheduler) AddJob(job *CronJob) error {
	if err := validateCronJob(job); err != nil {
		return err
	}
	job.SessionMode = NormalizeCronSessionMode(job.SessionMode)
	job.Model = strings.TrimSpace(job.Model)
	if _, err := cron.ParseStandard(job.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression %q: %w", job.CronExpr, err)
	}
	if err := cs.validatePersistentTarget(job.Project, job.SessionKey); err != nil {
		return err
	}
	if err := cs.validateModelSupport(job); err != nil {
		return err
	}
	if err := cs.store.Add(job); err != nil {
		return err
	}
	if job.Enabled {
		return cs.scheduleJob(job)
	}
	return nil
}

func (cs *CronScheduler) RemoveJob(id string) bool {
	cs.editMu.Lock()
	defer cs.editMu.Unlock()
	cs.mu.Lock()
	if entryID, ok := cs.entries[id]; ok {
		cs.cron.Remove(entryID)
		delete(cs.entries, id)
	}
	cs.mu.Unlock()
	return cs.store.Remove(id)
}

func (cs *CronScheduler) EnableJob(id string) error {
	cs.editMu.Lock()
	defer cs.editMu.Unlock()
	job := cs.store.Snapshot(id)
	if job == nil {
		return fmt.Errorf("job %q not found", id)
	}
	if err := cs.validatePersistentTarget(job.Project, job.SessionKey); err != nil {
		return err
	}
	if err := cs.validateModelSupport(job); err != nil {
		return err
	}
	if !cs.store.SetEnabled(id, true) {
		return fmt.Errorf("job %q not found", id)
	}
	job = cs.store.Snapshot(id)
	if job != nil {
		return cs.scheduleJob(job)
	}
	return nil
}

func (cs *CronScheduler) DisableJob(id string) error {
	cs.editMu.Lock()
	defer cs.editMu.Unlock()
	if !cs.store.SetEnabled(id, false) {
		return fmt.Errorf("job %q not found", id)
	}
	cs.mu.Lock()
	if entryID, ok := cs.entries[id]; ok {
		cs.cron.Remove(entryID)
		delete(cs.entries, id)
	}
	cs.mu.Unlock()
	return nil
}

// UpdateJob modifies one job field; edits affect the next run's snapshot.
func (cs *CronScheduler) UpdateJob(id string, field string, value any) error {
	return cs.UpdateJobFields(id, map[string]any{field: value})
}

// UpdateJobFields applies an API edit atomically, including model/exec changes.
func (cs *CronScheduler) UpdateJobFields(id string, updates map[string]any) error {
	cs.editMu.Lock()
	defer cs.editMu.Unlock()

	job, err := cs.store.updateFields(id, updates, func(candidate *CronJob) error {
		for field := range updates {
			switch field {
			case "cron_expr":
				if _, err := cron.ParseStandard(candidate.CronExpr); err != nil {
					return fmt.Errorf("invalid cron expression %q: %w", candidate.CronExpr, err)
				}
			case "mode":
				switch candidate.Mode {
				case "", "default", "bypassPermissions", "acceptEdits", "plan", "auto", "dontAsk":
				default:
					return fmt.Errorf("invalid mode %q (want default, bypassPermissions, acceptEdits, plan, auto, or dontAsk)", candidate.Mode)
				}
			case "session_mode":
				mode := NormalizeCronSessionMode(candidate.SessionMode)
				if mode != "" && mode != "new_per_run" {
					return fmt.Errorf("invalid session_mode %q (want reuse, new_per_run, or new-per-run)", candidate.SessionMode)
				}
			}
		}
		_, projectChanged := updates["project"]
		_, targetChanged := updates["session_key"]
		if candidate.Enabled || projectChanged || targetChanged {
			if err := cs.validatePersistentTarget(candidate.Project, candidate.SessionKey); err != nil {
				return err
			}
		}
		_, modelChanged := updates["model"]
		_, enabledChanged := updates["enabled"]
		if modelChanged || projectChanged || (enabledChanged && candidate.Enabled) {
			return cs.validateModelSupport(candidate)
		}
		return nil
	})
	if err != nil {
		return err
	}

	_, scheduleChanged := updates["cron_expr"]
	_, enabledChanged := updates["enabled"]
	if scheduleChanged || enabledChanged {
		if job.Enabled {
			if err := cs.scheduleJob(job); err != nil {
				return fmt.Errorf("reschedule failed: %w", err)
			}
		} else {
			cs.mu.Lock()
			if entryID, ok := cs.entries[id]; ok {
				cs.cron.Remove(entryID)
				delete(cs.entries, id)
			}
			cs.mu.Unlock()
		}
	}
	return nil
}

func (cs *CronScheduler) Store() *CronStore {
	return cs.store
}

// NextRun returns the next scheduled run time for a job, or zero if not scheduled.
func (cs *CronScheduler) NextRun(jobID string) time.Time {
	cs.mu.RLock()
	entryID, ok := cs.entries[jobID]
	cs.mu.RUnlock()
	if !ok {
		return time.Time{}
	}
	for _, e := range cs.cron.Entries() {
		if e.ID == entryID {
			return e.Next
		}
	}
	return time.Time{}
}

func (cs *CronScheduler) scheduleJob(job *CronJob) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Remove existing schedule if any
	if old, ok := cs.entries[job.ID]; ok {
		cs.cron.Remove(old)
	}

	jobID := job.ID
	entryID, err := cs.cron.AddFunc(job.CronExpr, func() {
		cs.executeJob(jobID)
	})
	if err != nil {
		return err
	}
	cs.entries[jobID] = entryID
	return nil
}

func (cs *CronScheduler) executeJob(jobID string) {
	job := cs.store.Snapshot(jobID)
	if job == nil {
		return
	}
	cs.runJob(job, false)
}

// RunJobNow triggers a persisted cron job immediately in the background.
// Disabled jobs are allowed to run manually; only scheduled executions enforce Enabled.
func (cs *CronScheduler) RunJobNow(id string) error {
	job := cs.store.Snapshot(id)
	if job == nil {
		return fmt.Errorf("%w: %q", ErrCronJobNotFound, id)
	}
	cs.mu.RLock()
	engine, ok := cs.engines[job.Project]
	cs.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrCronProjectNotFound, job.Project)
	}
	if err := cs.validateModelSupport(job); err != nil {
		return err
	}
	if err := engine.ValidatePersistentProactiveTarget(job.SessionKey); err != nil {
		return err
	}
	go cs.runJob(job, true)
	return nil
}

func (cs *CronScheduler) runJob(job *CronJob, manual bool) {
	if job == nil {
		return
	}
	if !manual && !job.Enabled {
		return
	}

	cs.mu.RLock()
	engine, ok := cs.engines[job.Project]
	cs.mu.RUnlock()

	if !ok {
		slog.Error("cron: project not found", "job", job.ID, "project", job.Project, "manual", manual)
		cs.store.MarkRun(job.ID, fmt.Errorf("project %q not found", job.Project))
		return
	}
	if err := engine.ValidatePersistentProactiveTarget(job.SessionKey); err != nil {
		cs.store.MarkRun(job.ID, err)
		slog.Error("cron: unsupported persistent target", "id", job.ID, "manual", manual, "error", err)
		return
	}

	slog.Info("cron: executing job", "id", job.ID, "project", job.Project, "manual", manual, "prompt", truncateStr(job.Prompt, 60))

	done := make(chan error, 1)
	go func() {
		done <- engine.ExecuteCronJob(job)
	}()

	var err error
	timeout := job.ExecutionTimeout()
	if timeout > 0 {
		select {
		case err = <-done:
		case <-time.After(timeout):
			err = fmt.Errorf("job timed out after %v", timeout)
		}
	} else {
		err = <-done
	}

	cs.store.MarkRun(job.ID, err)

	if err != nil {
		slog.Error("cron: job failed", "id", job.ID, "manual", manual, "error", err)
	} else {
		slog.Info("cron: job completed", "id", job.ID, "manual", manual)
	}
}

func (cs *CronScheduler) validatePersistentTarget(project, sessionKey string) error {
	cs.mu.RLock()
	engine := cs.engines[project]
	cs.mu.RUnlock()
	if engine == nil {
		return nil
	}
	return engine.ValidatePersistentProactiveTarget(sessionKey)
}

func (cs *CronScheduler) validateModelSupport(job *CronJob) error {
	if strings.TrimSpace(job.Model) == "" {
		return nil
	}
	cs.mu.RLock()
	engine := cs.engines[job.Project]
	cs.mu.RUnlock()
	// Offline projects are checked when an engine is available at execution.
	if engine == nil {
		return nil
	}
	if support, ok := engine.agent.(PerTurnModelSupporter); !ok || !support.SupportsPerTurnModel() {
		return fmt.Errorf("project %q: %w", job.Project, ErrModelOverrideUnsupported)
	}
	return nil
}

// mutePlatform wraps a Platform and discards all outgoing messages.
// Used for muted cron jobs that should execute without sending chat messages.
type mutePlatform struct {
	Platform
}

func (m *mutePlatform) Reply(_ context.Context, _ any, _ string) error { return nil }
func (m *mutePlatform) Send(_ context.Context, _ any, _ string) error  { return nil }

func GenerateCronID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Errorf("generate cron id: %w", err))
	}
	return hex.EncodeToString(b)
}

func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}

var cronWeekdays = map[Language][7]string{
	LangEnglish:            {"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"},
	LangChinese:            {"周日", "周一", "周二", "周三", "周四", "周五", "周六"},
	LangTraditionalChinese: {"週日", "週一", "週二", "週三", "週四", "週五", "週六"},
	LangJapanese:           {"日曜", "月曜", "火曜", "水曜", "木曜", "金曜", "土曜"},
	LangSpanish:            {"domingo", "lunes", "martes", "miércoles", "jueves", "viernes", "sábado"},
}

var cronMonths = map[Language][13]string{
	LangEnglish:            {"", "Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"},
	LangChinese:            {"", "1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	LangTraditionalChinese: {"", "1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	LangJapanese:           {"", "1月", "2月", "3月", "4月", "5月", "6月", "7月", "8月", "9月", "10月", "11月", "12月"},
	LangSpanish:            {"", "ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"},
}

func cronLangNames(lang Language) (weekdays [7]string, months [13]string) {
	if w, ok := cronWeekdays[lang]; ok {
		weekdays = w
	} else {
		weekdays = cronWeekdays[LangEnglish]
	}
	if m, ok := cronMonths[lang]; ok {
		months = m
	} else {
		months = cronMonths[LangEnglish]
	}
	return
}

func isZhLikeLang(lang Language) bool {
	return lang == LangChinese || lang == LangTraditionalChinese || lang == LangJapanese
}

// parseStep parses a cron step field like "*/5" and returns (5, true).
func parseStep(field string) (int, bool) {
	if !strings.HasPrefix(field, "*/") {
		return 0, false
	}
	var n int
	if _, err := fmt.Sscanf(field[2:], "%d", &n); err == nil && n > 0 {
		return n, true
	}
	return 0, false
}

// CronExprToHuman converts a standard 5-field cron expression to a human-readable string.
func CronExprToHuman(expr string, lang Language) string {
	fields := strings.Fields(expr)
	if len(fields) == 6 && (strings.HasPrefix(fields[0], "CRON_TZ=") || strings.HasPrefix(fields[0], "TZ=")) {
		fields = fields[1:]
	}
	if len(fields) != 5 {
		return expr
	}
	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]
	weekdays, months := cronLangNames(lang)
	cjk := isZhLikeLang(lang)
	allWild := dom == "*" && month == "*" && dow == "*"

	// Pure interval: */N * * * * → "Every N minutes"
	if minStep, ok := parseStep(minute); ok && hour == "*" && allWild {
		switch lang {
		case LangChinese:
			return fmt.Sprintf("每%d分钟", minStep)
		case LangTraditionalChinese:
			return fmt.Sprintf("每%d分鐘", minStep)
		case LangJapanese:
			return fmt.Sprintf("%d分ごと", minStep)
		case LangSpanish:
			return fmt.Sprintf("Cada %d min", minStep)
		default:
			return fmt.Sprintf("Every %d min", minStep)
		}
	}

	// Hour interval: M */N * * * → "Every N hours (:MM)"
	if hourStep, ok := parseStep(hour); ok && allWild {
		m := padZero(minute)
		if minute == "*" {
			m = "00"
		}
		switch lang {
		case LangChinese:
			return fmt.Sprintf("每%d小时 (:%s)", hourStep, m)
		case LangTraditionalChinese:
			return fmt.Sprintf("每%d小時 (:%s)", hourStep, m)
		case LangJapanese:
			return fmt.Sprintf("%d時間ごと (:%s)", hourStep, m)
		case LangSpanish:
			return fmt.Sprintf("Cada %d h (:%s)", hourStep, m)
		default:
			return fmt.Sprintf("Every %d h (:%s)", hourStep, m)
		}
	}

	var parts []string

	// Weekday
	if dow != "*" {
		if n, err := strconv.Atoi(dow); err == nil && n >= 0 && n <= 6 {
			if cjk {
				parts = append(parts, weekdays[n])
			} else {
				parts = append(parts, "Every "+weekdays[n])
			}
		} else {
			parts = append(parts, "weekday("+dow+")")
		}
	}

	// Month
	if month != "*" {
		if n, err := strconv.Atoi(month); err == nil && n >= 1 && n <= 12 {
			parts = append(parts, months[n])
		}
	}

	// Day of month
	if dom != "*" {
		if cjk {
			parts = append(parts, dom+"日")
		} else {
			parts = append(parts, "day "+dom)
		}
	}

	// Time
	if hour != "*" && minute != "*" {
		if minStep, ok := parseStep(minute); ok {
			switch lang {
			case LangChinese, LangTraditionalChinese:
				parts = append(parts, fmt.Sprintf("%s时 每%d分钟", padZero(hour), minStep))
			case LangJapanese:
				parts = append(parts, fmt.Sprintf("%s時 %d分ごと", padZero(hour), minStep))
			default:
				parts = append(parts, fmt.Sprintf("hour %s every %d min", padZero(hour), minStep))
			}
		} else {
			parts = append(parts, fmt.Sprintf("%s:%s", padZero(hour), padZero(minute)))
		}
	} else if hour != "*" {
		if cjk {
			parts = append(parts, hour+"時")
		} else {
			parts = append(parts, "hour "+hour)
		}
	} else if minute != "*" {
		if minStep, ok := parseStep(minute); ok {
			switch lang {
			case LangChinese:
				parts = append(parts, fmt.Sprintf("每%d分钟", minStep))
			case LangTraditionalChinese:
				parts = append(parts, fmt.Sprintf("每%d分鐘", minStep))
			case LangJapanese:
				parts = append(parts, fmt.Sprintf("%d分ごと", minStep))
			default:
				parts = append(parts, fmt.Sprintf("every %d min", minStep))
			}
		} else {
			switch lang {
			case LangChinese, LangTraditionalChinese:
				parts = append(parts, "每小时第"+minute+"分")
			case LangJapanese:
				parts = append(parts, "毎時"+minute+"分")
			default:
				parts = append(parts, "minute "+minute+" of every hour")
			}
		}
	}

	// Frequency hint
	if allWild {
		switch lang {
		case LangChinese, LangTraditionalChinese:
			return "每天 " + strings.Join(parts, " ")
		case LangJapanese:
			return "毎日 " + strings.Join(parts, " ")
		case LangSpanish:
			return "Diario " + strings.Join(parts, " ")
		default:
			return "Daily at " + strings.Join(parts, " ")
		}
	}
	if dow != "*" && month == "*" && dom == "*" {
		switch lang {
		case LangChinese, LangTraditionalChinese:
			return "每" + strings.Join(parts, " ")
		case LangJapanese:
			return "毎" + strings.Join(parts, " ")
		default:
			return strings.Join(parts, " at ")
		}
	}
	if dom != "*" && month == "*" && dow == "*" {
		switch lang {
		case LangChinese, LangTraditionalChinese:
			return "每月" + strings.Join(parts, " ")
		case LangJapanese:
			return "毎月" + strings.Join(parts, " ")
		case LangSpanish:
			return "Mensual, " + strings.Join(parts, ", ")
		default:
			return "Monthly, " + strings.Join(parts, ", ")
		}
	}

	if cjk {
		return strings.Join(parts, " ")
	}
	return strings.Join(parts, ", ")
}

func padZero(s string) string {
	if len(s) == 1 {
		return "0" + s
	}
	return s
}

// cronDisplayLocation shares the scheduler parser, including DST-aware zones.
func cronDisplayLocation(expr string) *time.Location {
	if schedule, err := cron.ParseStandard(expr); err == nil {
		if spec, ok := schedule.(*cron.SpecSchedule); ok && spec.Location != nil {
			return spec.Location
		}
	}
	return time.Local
}

func cronDisplaySchedule(expr string, lang Language) string {
	human := CronExprToHuman(expr, lang)
	fields := strings.Fields(expr)
	if len(fields) > 0 && (strings.HasPrefix(fields[0], "CRON_TZ=") || strings.HasPrefix(fields[0], "TZ=")) {
		if _, err := cron.ParseStandard(expr); err == nil {
			human += " (" + cronDisplayLocation(expr).String() + ")"
		}
	}
	return human
}

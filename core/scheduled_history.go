package core

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// ScheduledHistory observes the scheduler; jobs.json remains the scheduling source.
// One daemon instance owns the data directory. Query tools may open the WAL DB.
type ScheduledHistory struct {
	db        *sql.DB
	processID string
}

const scheduledHistorySchema = `
CREATE TABLE IF NOT EXISTS job_versions (
 version_id TEXT PRIMARY KEY, kind TEXT NOT NULL, job_id TEXT NOT NULL,
 project TEXT NOT NULL, description TEXT NOT NULL, observed_at TEXT NOT NULL,
 source TEXT NOT NULL, snapshot_json TEXT NOT NULL,
 archived_at TEXT, archive_reason TEXT
);
CREATE INDEX IF NOT EXISTS job_versions_by_job ON job_versions(kind, job_id, observed_at);
CREATE TABLE IF NOT EXISTS runs (
 run_id TEXT PRIMARY KEY, kind TEXT NOT NULL, job_id TEXT NOT NULL,
 version_id TEXT NOT NULL REFERENCES job_versions(version_id), process_id TEXT NOT NULL,
 project TEXT NOT NULL, session_key TEXT NOT NULL, trigger TEXT NOT NULL,
 started_at TEXT NOT NULL, scheduler_finished_at TEXT, scheduler_duration_ms INTEGER,
 scheduler_status TEXT NOT NULL, scheduler_error TEXT,
 execution_started_at TEXT, execution_finished_at TEXT, execution_duration_ms INTEGER,
 execution_status TEXT NOT NULL, execution_error TEXT,
 cc_session_id TEXT, agent_session_id TEXT, interrupted_observed_at TEXT
);
CREATE INDEX IF NOT EXISTS runs_by_job_time ON runs(kind, job_id, started_at);
CREATE INDEX IF NOT EXISTS runs_by_agent_session ON runs(agent_session_id);
CREATE TABLE IF NOT EXISTS legacy_events (
 event_id TEXT PRIMARY KEY, kind TEXT NOT NULL, job_id TEXT NOT NULL,
 occurred_at TEXT, event TEXT NOT NULL, source TEXT NOT NULL,
 run_id TEXT, facts_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS legacy_events_by_job ON legacy_events(kind, job_id, occurred_at);
CREATE TABLE IF NOT EXISTS evidence_refs (
 evidence_id TEXT PRIMARY KEY, reference_key TEXT NOT NULL,
 kind TEXT NOT NULL, job_id TEXT, run_id TEXT, cc_session_id TEXT,
 agent_session_id TEXT, thread_id TEXT, receipt_id TEXT, message_id TEXT,
 occurred_at TEXT, observed_at TEXT NOT NULL, source_path TEXT NOT NULL,
 source_sha256 TEXT NOT NULL, facts_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS evidence_by_reference ON evidence_refs(reference_key, observed_at);
CREATE INDEX IF NOT EXISTS evidence_by_run ON evidence_refs(run_id);
CREATE INDEX IF NOT EXISTS evidence_by_agent ON evidence_refs(agent_session_id);
PRAGMA user_version=1;
`

func NewScheduledHistory(dataDir string) (*ScheduledHistory, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "history.sqlite")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	if err = os.Chmod(path, 0o600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	failed := func(err error) (*ScheduledHistory, error) { _ = db.Close(); return nil, err }
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return failed(err)
	}
	if version > 1 {
		return failed(fmt.Errorf("scheduled history schema %d is newer than supported", version))
	}
	if _, err = db.Exec("PRAGMA busy_timeout=1000; PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		return failed(err)
	}
	if _, err = db.Exec(scheduledHistorySchema); err != nil {
		return failed(err)
	}
	processID, err := scheduledHistoryID()
	if err != nil {
		return failed(err)
	}
	return &ScheduledHistory{db: db, processID: processID}, nil
}

func (h *ScheduledHistory) Close() error { return h.db.Close() }

// RecoverInterrupted is called only at daemon startup, after its instance lock.
// Observation of a lost scheduler is not proof that an external agent finished.
func (h *ScheduledHistory) RecoverInterrupted() {
	_, err := h.db.Exec(`UPDATE runs SET scheduler_status='interrupted',
 interrupted_observed_at=?, execution_status=CASE WHEN execution_status='running' THEN 'unknown' ELSE execution_status END
 WHERE scheduler_status='running' AND process_id<>?`, historyTime(), h.processID)
	historyWarning("recover interrupted scheduler records", err)
}

func scheduledHistoryID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}

func historyTime() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func historyWarning(operation string, err error) {
	if err != nil {
		slog.Warn("scheduled history write failed", "operation", operation, "error", err)
	}
}

func historyDefinition(kind string, job any) (id, project, description, versionID string, body []byte, err error) {
	body, err = json.Marshal(job)
	if err != nil {
		return
	}
	var fields map[string]any
	if err = json.Unmarshal(body, &fields); err != nil {
		return
	}
	for _, key := range []string{"last_run", "last_error", "fired", "fired_at"} {
		delete(fields, key)
	}
	id, _ = fields["id"].(string)
	project, _ = fields["project"].(string)
	description, _ = fields["description"].(string)
	body, err = json.Marshal(fields)
	if err == nil {
		hash := sha256.Sum256(append([]byte(kind+"\x00"+id+"\x00"), body...))
		versionID = hex.EncodeToString(hash[:])
	}
	return
}

func (h *ScheduledHistory) Observe(kind string, job any) {
	if h == nil {
		return
	}
	id, project, description, version, body, err := historyDefinition(kind, job)
	if err == nil {
		_, err = h.db.Exec(`INSERT OR IGNORE INTO job_versions
 (version_id,kind,job_id,project,description,observed_at,source,snapshot_json) VALUES(?,?,?,?,?,?,?,?)`,
			version, kind, id, project, description, historyTime(), "runtime", string(body))
	}
	historyWarning("observe job definition", err)
}

func (h *ScheduledHistory) archive(kind, jobID string) {
	if h == nil {
		return
	}
	_, err := h.db.Exec(`UPDATE job_versions SET archived_at=COALESCE(archived_at,?),
 archive_reason=COALESCE(archive_reason,'removed_from_scheduler') WHERE kind=? AND job_id=?`, historyTime(), kind, jobID)
	historyWarning("archive removed job definition", err)
}

// scheduledRun is unique even when the same manual worker runs concurrently.
type scheduledRun struct {
	history   *ScheduledHistory
	id        string
	started   time.Time
	execution time.Time
}

func (h *ScheduledHistory) begin(kind string, job any, sessionKey string, manual bool) *scheduledRun {
	if h == nil {
		return nil
	}
	id, project, description, version, body, err := historyDefinition(kind, job)
	if err != nil {
		historyWarning("prepare run definition", err)
		return nil
	}
	runID, err := scheduledHistoryID()
	if err != nil {
		historyWarning("allocate run identity", err)
		return nil
	}
	started := time.Now()
	trigger := "scheduled"
	if manual {
		trigger = "manual"
	}
	tx, err := h.db.Begin()
	if err != nil {
		historyWarning("begin run transaction", err)
		return nil
	}
	// A committed transaction needs no rollback; other cleanup errors are diagnostic.
	defer func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			historyWarning("rollback run transaction", err)
		}
	}()
	_, err = tx.Exec(`INSERT OR IGNORE INTO job_versions
 (version_id,kind,job_id,project,description,observed_at,source,snapshot_json) VALUES(?,?,?,?,?,?,?,?)`,
		version, kind, id, project, description, started.UTC().Format(time.RFC3339Nano), "runtime", string(body))
	if err == nil {
		_, err = tx.Exec(`INSERT INTO runs
 (run_id,kind,job_id,version_id,process_id,project,session_key,trigger,started_at,scheduler_status,execution_status)
 VALUES(?,?,?,?,?,?,?,?,?,'running','not_started')`, runID, kind, id, version, h.processID, project, sessionKey, trigger,
			started.UTC().Format(time.RFC3339Nano))
	}
	if err == nil {
		err = tx.Commit()
	}
	if err != nil {
		historyWarning("persist run start", err)
		return nil
	}
	return &scheduledRun{history: h, id: runID, started: started}
}

func (r *scheduledRun) runID() string {
	if r == nil {
		return ""
	}
	return r.id
}

func (r *scheduledRun) executionStarted() {
	if r == nil {
		return
	}
	r.execution = time.Now()
	_, err := r.history.db.Exec(`UPDATE runs SET execution_started_at=?,execution_status='running' WHERE run_id=?`,
		r.execution.UTC().Format(time.RFC3339Nano), r.id)
	historyWarning("record engine dispatch", err)
}

func historyError(err error) any {
	if err == nil {
		return nil
	}
	return scheduledShellDiagnostic(err.Error())
}

func (r *scheduledRun) executionFinished(cause error) {
	if r == nil {
		return
	}
	status := "succeeded"
	if cause != nil {
		status = "failed"
	}
	_, err := r.history.db.Exec(`UPDATE runs SET execution_finished_at=?,execution_duration_ms=?,execution_status=?,execution_error=? WHERE run_id=?`,
		historyTime(), time.Since(r.execution).Milliseconds(), status, historyError(cause), r.id)
	historyWarning("record engine result", err)
}

func (r *scheduledRun) finish(status string, cause error) {
	if r == nil {
		return
	}
	_, err := r.history.db.Exec(`UPDATE runs SET scheduler_finished_at=?,scheduler_duration_ms=?,scheduler_status=?,scheduler_error=? WHERE run_id=?`,
		historyTime(), time.Since(r.started).Milliseconds(), status, historyError(cause), r.id)
	historyWarning("record scheduler result", err)
}

func (r *scheduledRun) linkSession(session *Session) {
	if r == nil || session == nil {
		return
	}
	_, err := r.history.db.Exec(`UPDATE runs SET cc_session_id=?,agent_session_id=? WHERE run_id=?`,
		session.ID, session.GetAgentSessionID(), r.id)
	historyWarning("link scheduled agent session", err)
}

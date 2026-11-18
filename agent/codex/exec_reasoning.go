package codex

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

// Each exec subprocess owns one turn. Its JSON projection can omit reasoning
// with empty summaries and assign different IDs to the items it does emit.
type execReasoning struct {
	sessions, resumeID, threadID string
	path, turnID, activeTurn     string
	before                       os.FileInfo
	started                      time.Time
	file                         *os.File
	reader                       *bufio.Reader
	partial                      []byte
	rolloutIDs, stdoutIDs        map[string]struct{}
	emitted                      int
	complete                     bool
}

func newExecReasoning(home, resumeID string) *execReasoning {
	r := &execReasoning{
		sessions: filepath.Join(resolveCodexHomeDir(home), "sessions"), resumeID: resumeID, started: time.Now(),
		rolloutIDs: make(map[string]struct{}), stdoutIDs: make(map[string]struct{}),
	}
	// Capture the boundary before Start: replaying the old turns of a
	// resumed thread would inflate every later card.
	if resumeID != "" {
		path := findReasoningRollout(resumeID, r.sessions)
		if info, err := os.Lstat(path); err == nil && info.Mode().IsRegular() {
			r.path, r.before = path, info
		}
	}
	return r
}

func (r *execReasoning) observe(event map[string]any) {
	switch event["type"] {
	case "thread.started":
		id, _ := event["thread_id"].(string)
		if r.threadID == "" && (r.resumeID == "" || r.resumeID == id) {
			r.threadID = id
		}
	case "item.completed":
		item, _ := event["item"].(map[string]any)
		id, _ := item["id"].(string)
		if item["type"] == "reasoning" && id != "" {
			r.stdoutIDs[id] = struct{}{}
		}
	}
}

func (cs *codexSession) emitExecReasoning(r *execReasoning) {
	// Both sources describe the same completions. A union would count a
	// visible item twice. stdout also works for clients without a rollout.
	for r.emitted < max(len(r.stdoutIDs), len(r.rolloutIDs)) {
		select {
		case cs.events <- core.Event{Type: core.EventThinking, ThinkingKind: core.ThinkingItem}:
			r.emitted++
		case <-cs.ctx.Done():
			return
		}
	}
}

func (r *execReasoning) poll(final bool) {
	if r.threadID == "" || (r.file == nil && !r.open(final)) {
		return
	}
	for {
		line, err := r.reader.ReadBytes('\n')
		if err != nil {
			// A trailing partial line belongs to the next poll.
			r.partial = append(r.partial, line...)
			return
		}
		if len(r.partial) > 0 {
			line = append(r.partial, line...)
			r.partial = nil
		}
		r.record(line)
	}
}

func (r *execReasoning) open(final bool) bool {
	if r.path == "" {
		if r.resumeID != "" {
			// There was no pre-launch boundary for this resumed thread.
			return false
		}
		// Codex creates a fresh rollout lazily using its own local date.
		// Check adjacent days for midnight and timezone differences.
		dirs := []string{r.sessions}
		if !final {
			dirs = nil
			for day := -1; day <= 1; day++ {
				date := r.started.AddDate(0, 0, day).Format("2006/01/02")
				dirs = append(dirs, filepath.Join(r.sessions, date))
			}
		}
		r.path = findReasoningRollout(r.threadID, dirs...)
	}
	info, err := os.Lstat(r.path)
	if err != nil || !info.Mode().IsRegular() || (r.before != nil && (!os.SameFile(info, r.before) || info.Size() < r.before.Size())) {
		return false
	}
	f, err := os.Open(r.path)
	if err != nil {
		return false
	}
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		_ = f.Close()
		return false
	}
	reader := bufio.NewReader(f)
	line, err := reader.ReadBytes('\n')
	var meta struct {
		Type    string
		Payload struct{ ID string }
	}
	if err != nil || json.Unmarshal(line, &meta) != nil || meta.Type != "session_meta" || meta.Payload.ID != r.threadID {
		_ = f.Close()
		return false
	}
	if r.resumeID != "" {
		if _, err := f.Seek(r.before.Size(), io.SeekStart); err != nil {
			_ = f.Close()
			return false
		}
		reader.Reset(f)
	}
	r.file, r.reader = f, reader
	return true
}

// History search also accepts partial filenames. The counter must select the
// rollout whose filename and first session_meta both identify this thread.
func findReasoningRollout(id string, dirs ...string) string {
	var found string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err == nil && entry.Type().IsRegular() && strings.HasPrefix(entry.Name(), "rollout-") &&
				strings.HasSuffix(entry.Name(), "-"+id+".jsonl") {
				if found != "" {
					return fs.ErrExist // Two candidates cannot identify this run.
				}
				found = path
			}
			return nil
		})
		if err != nil {
			return ""
		}
	}
	return found
}

func (r *execReasoning) record(line []byte) {
	// Skip often large tool inputs/outputs without decoding their contents.
	if !bytes.Contains(line, []byte(`"task_started"`)) && !bytes.Contains(line, []byte(`"task_complete"`)) &&
		!bytes.Contains(line, []byte(`"turn_context"`)) && !bytes.Contains(line, []byte(`"Reasoning"`)) &&
		!bytes.Contains(line, []byte(`"reasoning"`)) {
		return
	}
	var row struct {
		Type    string
		Payload struct {
			Type, ID, Status string
			TurnID           string `json:"turn_id"`
			ThreadID         string `json:"thread_id"`
			Item             struct{ Type, ID string }
		}
	}
	if json.Unmarshal(line, &row) != nil {
		return
	}
	p := row.Payload
	switch {
	case row.Type == "turn_context" || (row.Type == "event_msg" && p.Type == "task_started"):
		r.activeTurn = p.TurnID
		if r.turnID == "" {
			r.turnID = p.TurnID
		}
	case row.Type == "event_msg" && p.Type == "task_complete":
		if p.TurnID != "" && p.TurnID == r.turnID {
			r.complete = true
		}
	case row.Type == "event_msg" && p.Type == "item_completed" && p.Item.Type == "Reasoning":
		if p.ThreadID == r.threadID && p.TurnID != "" && p.TurnID == r.turnID && p.Item.ID != "" {
			r.rolloutIDs[p.Item.ID] = struct{}{}
		}
	case row.Type == "response_item" && p.Type == "reasoning" && (p.Status == "" || p.Status == "completed"):
		// Older rollouts have only response_item. Newer ones mirror the
		// explicit completion with the same native item ID.
		if r.turnID != "" && r.activeTurn == r.turnID && p.ID != "" {
			r.rolloutIDs[p.ID] = struct{}{}
		}
	}
}

func (r *execReasoning) close() {
	if r.file != nil {
		_ = r.file.Close()
	}
}

package codex

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func TestTransportRecoveryApprovalsCannotCrossReusedRequestID(t *testing.T) {
	for _, kind := range []string{"command", "permissions", "user-input"} {
		t.Run(kind, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			s := &appServerSession{ctx: ctx, cancel: cancel, events: make(chan core.Event, 8), pendingApprovals: make(map[string]chan core.PermissionResult)}
			s.transportGeneration.Store(1)
			request := func() {
				switch kind {
				case "command":
					s.handleApprovalRequest(json.RawMessage(`7`), "item/commandExecution/requestApproval", json.RawMessage(`{"command":"fixture"}`))
				case "permissions":
					s.handlePermissionsApproval(json.RawMessage(`7`), json.RawMessage(`{"permissions":{"network":{}}}`))
				case "user-input":
					s.handleRequestUserInput(json.RawMessage(`7`), json.RawMessage(`{"questions":[{"id":"q","header":"Choice","question":"Choose?","options":[{"label":"A","description":"A"}]}]}`))
				}
			}
			request()
			old := <-s.events
			// Old readers have stopped and stdin is detached, as required by the helper.
			if err := s.drainTransportApprovals(); err != nil {
				t.Fatal(err)
			}
			replacement := &lockedWriteCloser{}
			s.procMu.Lock()
			s.stdin = replacement
			s.procMu.Unlock()
			s.transportGeneration.Add(1)
			request()
			fresh := <-s.events
			if old.RequestID == fresh.RequestID {
				t.Fatal("replacement reused the UI approval identity")
			}
			if err := s.RespondPermission(old.RequestID, core.PermissionResult{Behavior: "allow"}); err == nil {
				t.Fatal("late old approval accepted for new request")
			}
			if replacement.String() != "" {
				t.Fatal("old response reached replacement transport")
			}
			if err := s.RespondPermission(fresh.RequestID, core.PermissionResult{Behavior: "deny"}); err != nil {
				t.Fatal(err)
			}
			s.approvalWorkers.Wait()
			var response struct {
				ID int `json:"id"`
			}
			if err := json.Unmarshal([]byte(replacement.String()), &response); err != nil || response.ID != 7 {
				t.Fatalf("native response identity changed: %q (%v)", replacement.String(), err)
			}
		})
	}
}

type approvalGateWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (w *approvalGateWriter) Write(p []byte) (int, error) {
	w.once.Do(func() { close(w.entered) })
	<-w.release
	return len(p), nil
}
func (w *approvalGateWriter) Close() error { return nil }

func TestTransportRecoveryWaitsForApprovalResponseAlreadyWriting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	writer := &approvalGateWriter{entered: make(chan struct{}), release: make(chan struct{})}
	s := &appServerSession{ctx: ctx, cancel: cancel, stdin: writer, events: make(chan core.Event, 1), pendingApprovals: make(map[string]chan core.PermissionResult)}
	s.handleApprovalRequest(json.RawMessage(`7`), "item/commandExecution/requestApproval", json.RawMessage(`{"command":"fixture"}`))
	event := <-s.events
	if err := s.RespondPermission(event.RequestID, core.PermissionResult{Behavior: "allow"}); err != nil {
		t.Fatal(err)
	}
	<-writer.entered
	s.procMu.Lock()
	s.stdin = nil
	s.procMu.Unlock()
	drained := make(chan error, 1)
	go func() { drained <- s.drainTransportApprovals() }()
	select {
	case err := <-drained:
		t.Fatalf("drain finished with old response in flight: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(writer.release)
	select {
	case err := <-drained:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("drain did not finish after old writer exited")
	}
}

func TestTransportRecoveryApprovalDrainHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &appServerSession{ctx: ctx, pendingApprovals: make(map[string]chan core.PermissionResult)}
	s.approvalWorkers.Add(1)
	defer s.approvalWorkers.Done()
	cancel()
	if err := s.drainTransportApprovals(); err != context.Canceled {
		t.Fatalf("drain cancellation = %v", err)
	}
}

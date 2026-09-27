package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

func newTerminalTestSession(t *testing.T) *appServerSession {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	s := &appServerSession{ctx: ctx, cancel: cancel, events: make(chan core.Event, 128), currentTurn: "turn-1"}
	s.threadID.Store("thread-1")
	s.alive.Store(true)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func awaitTerminalTest(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("app-server operation did not finish promptly")
	}
}

func TestAppServerSession_Issue138ActiveEOFFinishesTurn(t *testing.T) {
	s := newTerminalTestSession(t)
	rpc := make(chan rpcResponseEnvelope, 1)
	s.pending = map[int64]chan rpcResponseEnvelope{1: rpc}
	s.wg.Add(1)
	s.readLoop(strings.NewReader(""))
	select {
	case event := <-s.events:
		if event.Type != core.EventError || !errors.Is(event.Error, io.EOF) {
			t.Fatalf("EOF event = %#v, want terminal EOF error", event)
		}
	default:
		t.Fatal("EOF left the foreground turn waiting without a terminal event")
	}
	if s.Alive() || appServerCurrentTurn(s) != "" {
		t.Fatal("EOF left the transport or turn active")
	}
	if response := <-rpc; !response.localFailure {
		t.Fatal("EOF did not reject pending RPCs as transport failures")
	}
	d := s.DiagnosticSnapshot()
	if d.ReadState != "eof" || *d.TerminalReceived || !*d.TerminalDelivered {
		t.Fatalf("EOF diagnostics = %#v", d)
	}
}

func TestAppServerSession_Issue138EOFDoesNotDuplicateTerminal(t *testing.T) {
	for _, notification := range []string{"idle", "completed", "error", "cancelled"} {
		t.Run(notification, func(t *testing.T) {
			s := newTerminalTestSession(t)
			switch notification {
			case "idle":
				s.currentTurn = ""
			case "completed":
				s.handleNotification("turn/completed", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1"}}`))
			case "error":
				s.handleNotification("error", json.RawMessage(`{"threadId":"thread-1","turnId":"turn-1","message":"failed"}`))
			case "cancelled":
				s.cancel()
			}
			before := len(s.events)
			s.wg.Add(1)
			s.readLoop(strings.NewReader(""))
			if len(s.events) != before {
				t.Fatalf("EOF emitted an extra terminal event after %s", notification)
			}
		})
	}
}

func TestAppServerSession_Issue138TerminalSurvivesFullQueue(t *testing.T) {
	for _, terminal := range []core.Event{{Type: core.EventResult, Done: true}, {Type: core.EventError, Error: io.EOF}, {Type: core.EventText, Done: true}} {
		t.Run(string(terminal.Type), func(t *testing.T) {
			s := newTerminalTestSession(t)
			for i := 0; i < cap(s.events); i++ {
				s.events <- core.Event{Type: core.EventThinking, Content: "queued"}
			}
			done := make(chan struct{})
			go func() { s.emit(terminal); close(done) }()
			select {
			case <-done:
				t.Fatal("terminal delivery returned while the queue was still full")
			case <-time.After(20 * time.Millisecond):
			}
			// Reading diagnostics must remain possible while delivery waits for
			// the consumer; the engine takes snapshots while draining events.
			snapshotDone := make(chan struct{})
			go func() { _ = s.DiagnosticSnapshot(); close(snapshotDone) }()
			awaitTerminalTest(t, snapshotDone)
			for i := 0; i < cap(s.events); i++ {
				if event := <-s.events; event.Type != core.EventThinking || event.Content != "queued" {
					t.Fatalf("terminal displaced or reordered queued event %d: %#v", i, event)
				}
			}
			awaitTerminalTest(t, done)
			select {
			case event := <-s.events:
				if event.Type != terminal.Type || event.Done != terminal.Done || event.Error != terminal.Error {
					t.Fatalf("terminal = %#v, want %#v", event, terminal)
				}
			default:
				t.Fatal("full event queue dropped the terminal event")
			}
			d := s.DiagnosticSnapshot()
			if !*d.TerminalDelivered || *d.DroppedEvents != 0 {
				t.Fatalf("terminal delivery diagnostics = %#v", d)
			}
		})
	}
}

func TestAppServerSession_Issue138CloseUnblocksTerminalDelivery(t *testing.T) {
	s := newTerminalTestSession(t)
	for i := 0; i < cap(s.events); i++ {
		s.events <- core.Event{Type: core.EventThinking}
	}
	done := make(chan struct{})
	s.wg.Add(1)
	go func() { defer s.wg.Done(); s.emitError(io.EOF); close(done) }()
	select {
	case <-done:
		t.Fatal("terminal delivery returned before cancellation or queue space")
	case <-time.After(20 * time.Millisecond):
	}
	closed := make(chan struct{})
	go func() { _ = s.Close(); close(closed) }()
	awaitTerminalTest(t, done)
	awaitTerminalTest(t, closed)
	// Close can race with publishers that are not owned by the read loop.
	s.emitError(io.EOF)
}

func TestAppServerSession_Issue138StartResponseCannotReviveFinishedTurn(t *testing.T) {
	for _, completed := range []bool{false, true} {
		t.Run(map[bool]string{false: "eof", true: "completed_then_eof"}[completed], func(t *testing.T) {
			s := newTerminalTestSession(t)
			s.currentTurn = ""
			s.workDir = t.TempDir()
			s.stdin = &steerFakeStdin{respond: func(id int64, _ map[string]any) {
				s.handleResponse(rpcResponseEnvelope{ID: id, Result: json.RawMessage(`{"turn":{"id":"turn-1"}}`)})
				if completed {
					s.handleNotification("turn/started", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1"}}`))
					s.handleNotification("turn/completed", json.RawMessage(`{"threadId":"thread-1","turn":{"id":"turn-1"}}`))
				}
				s.wg.Add(1)
				s.readLoop(strings.NewReader(""))
			}}
			err := s.Send("hello", nil, nil)
			if completed && err != nil {
				t.Fatalf("completed turn became a transport failure: %v", err)
			}
			if !completed && err == nil {
				t.Fatal("turn/start succeeded after transport EOF with no terminal event")
			}
			if appServerCurrentTurn(s) != "" {
				t.Fatal("late turn/start response revived a finished turn")
			}
		})
	}
}

func TestAppServerSession_Issue138EOFRejectsRPCBeforeBlockedTerminal(t *testing.T) {
	s := newTerminalTestSession(t)
	for i := 0; i < cap(s.events); i++ {
		s.events <- core.Event{Type: core.EventThinking}
	}
	rpc := make(chan rpcResponseEnvelope, 1)
	s.pending = map[int64]chan rpcResponseEnvelope{1: rpc}
	done := make(chan struct{})
	s.wg.Add(1)
	go func() { s.readLoop(strings.NewReader("")); close(done) }()
	select {
	case response := <-rpc:
		if !response.localFailure || s.Alive() {
			t.Fatal("RPC did not observe a dead transport")
		}
	case <-time.After(time.Second):
		t.Fatal("terminal backpressure prevented pending RPC rejection")
	}
	for i := 0; i < cap(s.events); i++ {
		<-s.events
	}
	awaitTerminalTest(t, done)
	select {
	case event := <-s.events:
		if event.Type != core.EventError || !errors.Is(event.Error, io.EOF) {
			t.Fatalf("EOF terminal = %#v", event)
		}
	default:
		t.Fatal("EOF terminal was lost under backpressure")
	}
}

// A real local pipe/process boundary exercises stdout draining and cmd.Wait
// without invoking Codex, a provider, or any messaging platform.
func TestAppServerSession_Issue138ProcessExitFinishesOnce(t *testing.T) {
	for _, mode := range []string{"eof", "completed", "failed", "completed_then_failed_exit"} {
		t.Run(mode, func(t *testing.T) {
			s := newTerminalTestSession(t)
			cmd := exec.Command(os.Args[0], "-test.run=^TestAppServerSession_Issue138ProcessHelper$")
			cmd.Env = append(os.Environ(), "CCN138_PROCESS_HELPER="+mode, "GORACE=atexit_sleep_ms=0")
			stdout, err := cmd.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			s.cmd = cmd
			s.readDone = make(chan struct{})
			s.wg.Add(2)
			done := make(chan struct{})
			go func() { s.waitLoop(); close(done) }()
			go s.readLoop(stdout)
			awaitTerminalTest(t, done)
			want := core.EventError
			if strings.HasPrefix(mode, "completed") {
				want = core.EventResult
			}
			if len(s.events) != 1 {
				t.Fatalf("process exit produced %d terminal events, want 1", len(s.events))
			}
			if event := <-s.events; event.Type != want {
				t.Fatalf("process exit event = %#v, want %v", event, want)
			}
			if s.Alive() || appServerCurrentTurn(s) != "" || s.DiagnosticSnapshot().ExitCode == nil {
				t.Fatal("process exit left incomplete lifecycle diagnostics")
			}
		})
	}
}

func TestAppServerSession_Issue138ProcessHelper(t *testing.T) {
	mode := os.Getenv("CCN138_PROCESS_HELPER")
	if mode == "" {
		return
	}
	if strings.HasPrefix(mode, "completed") {
		fmt.Fprintln(os.Stdout, `{"method":"turn/completed","params":{"threadId":"thread-1","turn":{"id":"turn-1"}}}`)
	}
	if mode == "failed" || mode == "completed_then_failed_exit" {
		os.Exit(17)
	}
	os.Exit(0)
}

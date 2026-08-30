package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/timmyagentic/cc-connect-next/core"
)

type rpcResponseEnvelope struct {
	ID     any             `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
	// localFailure marks envelopes synthesized by rejectPending when the
	// transport dies, as opposed to real server responses. Callers that need
	// to distinguish "server rejected" from "outcome unknown" check this.
	localFailure bool `json:"-"`
}

type rpcNotificationEnvelope struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type initResponse struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type threadStartResponse struct {
	Cwd             string  `json:"cwd"`
	Model           string  `json:"model"`
	ReasoningEffort *string `json:"reasoningEffort"`
	Thread          struct {
		ID string `json:"id"`
	} `json:"thread"`
}

type threadResumeResponse struct {
	Cwd             string  `json:"cwd"`
	Model           string  `json:"model"`
	ReasoningEffort *string `json:"reasoningEffort"`
	Thread          struct {
		ID string `json:"id"`
	} `json:"thread"`
}

type turnStartResponse struct {
	Turn struct {
		ID string `json:"id"`
	} `json:"turn"`
}

// turnSteerResponse decodes the turn/steer result. The app-server returns the
// id of the turn the input was appended to as a flat "turnId"; a nested
// "turn.id" is tolerated for forward compatibility.
type turnSteerResponse struct {
	TurnID string `json:"turnId"`
	Turn   struct {
		ID string `json:"id"`
	} `json:"turn"`
}

func (r turnSteerResponse) turnID() string {
	if r.TurnID != "" {
		return r.TurnID
	}
	return r.Turn.ID
}

type turnNotification struct {
	ThreadID string `json:"threadId"`
	Turn     struct {
		ID     string `json:"id"`
		Status string `json:"status"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	} `json:"turn"`
}

type itemNotification struct {
	ThreadID string         `json:"threadId"`
	TurnID   string         `json:"turnId"`
	Item     map[string]any `json:"item"`
}

type errorNotification struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
	Message  string `json:"message"` // legacy app-server payload
	Error    *struct {
		Message string `json:"message"`
	} `json:"error"`
	WillRetry bool `json:"willRetry"`
}

func (n errorNotification) errorMessage() string {
	if n.Error != nil && strings.TrimSpace(n.Error.Message) != "" {
		return n.Error.Message
	}
	return n.Message
}

type appServerMessageRoute struct {
	ThreadID string `json:"threadId"`
	TurnID   string `json:"turnId"`
}

type appServerRequestUserInputParams struct {
	ThreadID  string                              `json:"threadId"`
	TurnID    string                              `json:"turnId"`
	ItemID    string                              `json:"itemId"`
	Questions []appServerRequestUserInputQuestion `json:"questions"`
}

type appServerRequestUserInputQuestion struct {
	ID       string                            `json:"id"`
	Header   string                            `json:"header"`
	Question string                            `json:"question"`
	IsOther  bool                              `json:"isOther"`
	IsSecret bool                              `json:"isSecret"`
	Options  []appServerRequestUserInputOption `json:"options"`
}

type appServerRequestUserInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}

type appServerRequestUserInputResponse struct {
	Answers map[string]appServerRequestUserInputAnswer `json:"answers"`
}

type appServerRequestUserInputAnswer struct {
	Answers []string `json:"answers"`
}

type appServerSession struct {
	url                string
	cliBin             string   // CLI binary from the cmd option, default "codex"
	cliExtraArgs       []string // extra args from cmd, placed before the app-server subcommand
	workDir            string
	model              string
	contextWindow      int64
	effort             string
	serviceTier        string // Codex service tier, e.g. "fast"; catalog-driven, passed through verbatim
	mode               string
	baseURL            string
	modelProvider      string
	extraEnv           []string
	codexHome          string
	promptPreamble     string
	sessionTitlePrefix string
	sessionTitleModel  string
	titleGenerator     sessionTitleGenerator

	events chan core.Event

	ctx    context.Context
	cancel context.CancelFunc

	cmd     *exec.Cmd
	stdin   io.WriteCloser
	procMu  sync.Mutex
	writeMu sync.Mutex

	nextID atomic.Int64

	pendingMu sync.Mutex
	pending   map[int64]chan rpcResponseEnvelope

	approvalsMu      sync.Mutex
	pendingApprovals map[string]chan core.PermissionResult

	threadID atomic.Value
	alive    atomic.Bool

	closeOnce sync.Once
	wg        sync.WaitGroup

	stateMu      sync.Mutex
	pendingMsgs  []string
	currentTurn  string
	preambleSent bool
	// initialTitleHandled prevents later turns from replacing a fresh thread's
	// first user-facing title. Resumed threads start with this already set.
	initialTitleHandled bool
	titleMu             sync.Mutex
	explicitTitleRev    uint64

	runtimeMu   sync.RWMutex
	turnOptions *core.TurnOptions
}

const (
	appServerRequestTimeout       = 120 * time.Second
	appServerResponseWriteTimeout = 1500 * time.Millisecond
	appServerTitleUpdateTimeout   = 1500 * time.Millisecond
)

// appServerSessionParams bundles the launch configuration for a Codex
// app-server session. The previous positional-string signature let call sites
// silently drop fields — that is exactly how the cmd binary and extra args
// were lost on this backend (issue #37).
type appServerSessionParams struct {
	url                string
	cliBin             string   // CLI binary from the cmd option, default "codex"
	cliExtraArgs       []string // extra args from cmd, placed before the app-server subcommand
	workDir            string
	model              string
	contextWindow      int64
	effort             string
	serviceTier        string
	mode               string
	resumeID           string
	baseURL            string
	modelProvider      string
	extraEnv           []string
	codexHome          string
	systemPrompt       string
	appendPrompt       string
	sessionTitlePrefix string
	sessionTitleModel  string
}

func newAppServerSession(ctx context.Context, p appServerSessionParams) (*appServerSession, error) {
	cliBin := strings.TrimSpace(p.cliBin)
	if cliBin == "" {
		cliBin = "codex"
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	s := &appServerSession{
		url:                 p.url,
		cliBin:              cliBin,
		cliExtraArgs:        append([]string(nil), p.cliExtraArgs...),
		workDir:             p.workDir,
		model:               p.model,
		contextWindow:       p.contextWindow,
		effort:              p.effort,
		serviceTier:         p.serviceTier,
		mode:                p.mode,
		baseURL:             p.baseURL,
		modelProvider:       p.modelProvider,
		extraEnv:            append([]string(nil), p.extraEnv...),
		codexHome:           strings.TrimSpace(p.codexHome),
		promptPreamble:      buildCodexPromptPreamble(p.systemPrompt, p.appendPrompt),
		sessionTitlePrefix:  normalizeSessionTitlePrefix(p.sessionTitlePrefix),
		sessionTitleModel:   strings.TrimSpace(p.sessionTitleModel),
		events:              make(chan core.Event, 128),
		ctx:                 sessionCtx,
		cancel:              cancel,
		pending:             make(map[int64]chan rpcResponseEnvelope),
		pendingApprovals:    make(map[string]chan core.PermissionResult),
		preambleSent:        p.resumeID != "" && p.resumeID != core.ContinueSession,
		initialTitleHandled: p.resumeID != "" && p.resumeID != core.ContinueSession,
	}
	s.alive.Store(true)

	if err := s.connect(); err != nil {
		cancel()
		return nil, err
	}

	if err := s.initialize(); err != nil {
		_ = s.Close()
		return nil, err
	}

	if err := s.ensureThread(p.resumeID); err != nil {
		_ = s.Close()
		return nil, err
	}

	return s, nil
}

// launchArgs builds the argv (after the binary) used to spawn the Codex
// app-server. cliExtraArgs from the user's cmd option go before the
// app-server subcommand — the same global-flag position the exec backend uses
// — so the structured options emitted after them win on duplicate -c keys
// (codex resolves -c last-wins).
func (s *appServerSession) launchArgs() []string {
	args := append([]string(nil), s.cliExtraArgs...)
	args = append(args, "app-server")
	if url := strings.TrimSpace(s.url); url != "" {
		args = append(args, "--listen", url)
	}
	if model := strings.TrimSpace(s.model); model != "" {
		args = append(args, "-c", fmt.Sprintf("model=%q", model))
	}
	if s.contextWindow > 0 {
		args = append(args, "-c", fmt.Sprintf("model_context_window=%d", s.contextWindow))
	}
	if effort := strings.TrimSpace(s.effort); effort != "" {
		args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%q", effort))
	}
	if tier := strings.TrimSpace(s.serviceTier); tier != "" {
		args = append(args, "-c", fmt.Sprintf("service_tier=%q", tier))
	}
	if provider := strings.TrimSpace(s.modelProvider); provider != "" {
		args = append(args, "-c", fmt.Sprintf("model_provider=%q", provider))
	}
	if baseURL := strings.TrimSpace(s.baseURL); baseURL != "" {
		args = append(args, "-c", fmt.Sprintf("openai_base_url=%q", baseURL))
	}
	return args
}

func (s *appServerSession) connect() error {
	args := s.launchArgs()
	cmd := exec.CommandContext(s.ctx, s.cliBin, args...)
	cmd.Dir = s.workDir
	env := append([]string(nil), s.extraEnv...)
	if s.codexHome != "" {
		env = append(env, "CODEX_HOME="+s.codexHome)
	}
	if len(env) > 0 {
		cmd.Env = core.MergeEnv(os.Environ(), env)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("codex app-server stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("codex app-server stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("codex app-server stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("codex app-server start: %w", err)
	}

	s.procMu.Lock()
	s.cmd = cmd
	s.stdin = stdin
	s.procMu.Unlock()

	slog.Info("codex app-server session started", "transport", "stdio", "pid", cmd.Process.Pid, "work_dir", s.workDir)

	s.wg.Add(3)
	go s.readLoop(stdout)
	go s.stderrLoop(stderr)
	go s.waitLoop()
	return nil
}

func (s *appServerSession) initialize() error {
	params := map[string]any{
		"clientInfo": map[string]any{
			"name":    "cc-connect-next-codex-agent",
			"title":   "CC Connect Codex Agent",
			"version": "0.1.0",
		},
		"capabilities": map[string]any{
			"experimentalApi": true,
			"optOutNotificationMethods": []string{
				"command/exec/outputDelta",
				"item/agentMessage/delta",
				"item/plan/delta",
				"item/fileChange/outputDelta",
				"item/reasoning/summaryTextDelta",
				"item/reasoning/textDelta",
			},
		},
	}

	var resp initResponse
	if err := s.request("initialize", params, &resp); err != nil {
		return fmt.Errorf("codex app-server initialize: %w", err)
	}
	if err := s.notify("initialized", nil); err != nil {
		return fmt.Errorf("codex app-server initialized notify: %w", err)
	}
	return nil
}

func (s *appServerSession) ensureThread(resumeID string) error {
	if resumeID != "" && resumeID != core.ContinueSession {
		params := s.threadRequestParams()
		params["threadId"] = resumeID
		params["persistExtendedHistory"] = true

		var resp threadResumeResponse
		if err := s.request("thread/resume", params, &resp); err != nil {
			return err
		}
		if resp.Thread.ID == "" {
			return fmt.Errorf("codex app-server resume returned empty thread id")
		}
		s.applyThreadRuntimeState(resp.Cwd, resp.Model, resp.ReasoningEffort)
		s.threadID.Store(resp.Thread.ID)
		slog.Info("codex app-server thread resumed", "thread_id", resp.Thread.ID)
		return nil
	}

	var resp threadStartResponse
	if err := s.request("thread/start", s.threadRequestParams(), &resp); err != nil {
		return err
	}
	if resp.Thread.ID == "" {
		return fmt.Errorf("codex app-server start returned empty thread id")
	}
	s.applyThreadRuntimeState(resp.Cwd, resp.Model, resp.ReasoningEffort)
	s.threadID.Store(resp.Thread.ID)
	slog.Info("codex app-server thread started", "thread_id", resp.Thread.ID)
	return nil
}

func (s *appServerSession) threadRequestParams() map[string]any {
	params := map[string]any{
		"experimentalRawEvents":  false,
		"persistExtendedHistory": false,
	}
	if model := s.GetModel(); model != "" {
		params["model"] = model
	}
	if approval, sandbox := appServerModeSettings(s.mode); approval != "" {
		params["approvalPolicy"] = approval
		if sandbox != "" {
			params["sandbox"] = sandbox
		}
	}
	return params
}

func appServerModeSettings(mode string) (approval string, sandbox string) {
	switch normalizeMode(mode) {
	case "auto-edit", "full-auto":
		return "never", "workspace-write"
	case "yolo":
		return "never", "danger-full-access"
	default:
		return "on-request", "read-only"
	}
}

func (s *appServerSession) applyThreadRuntimeState(workDir, model string, effort *string) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if dir := strings.TrimSpace(workDir); dir != "" {
		s.workDir = dir
	}
	if m := strings.TrimSpace(model); m != "" {
		s.model = m
	}
	s.effort = normalizeRuntimeReasoningEffort(stringValue(effort))
}

func (s *appServerSession) Send(prompt string, images []core.ImageAttachment, files []core.FileAttachment) error {
	return s.send(prompt, images, files, nil)
}

func (s *appServerSession) SendWithTurnOptions(prompt string, images []core.ImageAttachment, files []core.FileAttachment, options core.TurnOptions) error {
	return s.send(prompt, images, files, &options)
}

func (s *appServerSession) send(prompt string, images []core.ImageAttachment, files []core.FileAttachment, options *core.TurnOptions) error {
	if !s.alive.Load() {
		return fmt.Errorf("session is closed")
	}

	if len(files) > 0 {
		filePaths := core.SaveFilesToDisk(s.workDir, files)
		prompt = core.AppendFileRefs(prompt, filePaths)
	}

	prompt, imagePaths, err := s.stageImages(prompt, images)
	if err != nil {
		return err
	}

	s.stateMu.Lock()
	if !s.preambleSent {
		prompt = prependCodexPromptPreamble(prompt, s.promptPreamble)
		s.preambleSent = true
	}
	s.stateMu.Unlock()

	threadID := s.CurrentSessionID()
	if threadID == "" {
		return fmt.Errorf("codex app-server thread id is empty")
	}

	input := make([]map[string]any, 0, 1+len(imagePaths))
	input = append(input, map[string]any{
		"type":          "text",
		"text":          prompt,
		"text_elements": []any{},
	})
	for _, path := range imagePaths {
		input = append(input, map[string]any{
			"type": "localImage",
			"path": path,
		})
	}

	params := s.turnStartParams(threadID, input, options)

	var resp turnStartResponse
	if err := s.request("turn/start", params, &resp); err != nil {
		return fmt.Errorf("codex app-server turn/start: %w", err)
	}
	if resp.Turn.ID == "" {
		return fmt.Errorf("codex app-server turn/start returned empty turn id")
	}

	s.stateMu.Lock()
	s.currentTurn = resp.Turn.ID
	s.pendingMsgs = s.pendingMsgs[:0]
	s.stateMu.Unlock()
	s.storeActiveTurnOptions(options)

	return nil
}

// SetSessionTitle persists a user-facing Codex thread name. Naming is
// metadata-only and intentionally separate from turn input.
func (s *appServerSession) SetSessionTitle(sessionID, title string) error {
	sessionID = strings.TrimSpace(sessionID)
	title = strings.TrimSpace(title)
	if sessionID == "" {
		return fmt.Errorf("codex app-server thread id is empty")
	}
	if title == "" {
		return fmt.Errorf("codex app-server thread title is empty")
	}
	title = formatSessionTitle(s.sessionTitlePrefix, title)
	if !s.alive.Load() {
		return fmt.Errorf("session is closed")
	}

	s.titleMu.Lock()
	defer s.titleMu.Unlock()
	if sessionID == s.CurrentSessionID() {
		s.explicitTitleRev++
		s.stateMu.Lock()
		s.initialTitleHandled = true
		s.stateMu.Unlock()
	}

	return s.setSessionTitleRPC(sessionID, title)
}

func (s *appServerSession) setSessionTitleRPC(sessionID, title string) error {
	var resp struct{}
	if err := s.requestWithTimeout("thread/name/set", map[string]any{
		"threadId": sessionID,
		"name":     title,
	}, &resp, appServerTitleUpdateTimeout); err != nil {
		return fmt.Errorf("codex app-server thread/name/set: %w", err)
	}
	return nil
}

// SetInitialSessionTitle names a fresh app-server thread at the creation
// boundary. Send intentionally does not own this lifecycle step.
func (s *appServerSession) SetInitialSessionTitle(prompt string) error {
	threadID := s.CurrentSessionID()
	if threadID == "" {
		return fmt.Errorf("codex app-server thread id is empty")
	}
	s.titleMu.Lock()
	s.stateMu.Lock()
	if s.initialTitleHandled {
		s.stateMu.Unlock()
		s.titleMu.Unlock()
		return nil
	}
	// Consume the one-shot attempt before optional generation so concurrent or
	// duplicate initialization can never launch a second model process.
	s.initialTitleHandled = true
	explicitTitleRev := s.explicitTitleRev
	s.stateMu.Unlock()
	s.titleMu.Unlock()

	title := initialCodexThreadTitle(prompt)
	if model := strings.TrimSpace(s.sessionTitleModel); model != "" {
		generator := s.titleGenerator
		if generator == nil {
			generator = s.generateSessionTitleWithCodex
		}
		ctx := s.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		generated, err := generator(ctx, title)
		if err != nil {
			slog.Warn("codex app-server: optional session title generation failed, using local title",
				"model", model, "error", err)
		} else if generated = initialCodexThreadTitle(generated); generated != untitledCodexThread {
			title = generated
		}
	}

	title = formatSessionTitle(s.sessionTitlePrefix, title)
	if !s.alive.Load() {
		return fmt.Errorf("session is closed")
	}

	s.titleMu.Lock()
	defer s.titleMu.Unlock()
	if s.explicitTitleRev != explicitTitleRev {
		return nil
	}
	return s.setSessionTitleRPC(threadID, title)
}

func (s *appServerSession) turnStartParams(threadID string, input []map[string]any, options *core.TurnOptions) map[string]any {
	params := map[string]any{
		"threadId": threadID,
		"input":    input,
	}
	if options == nil {
		s.runtimeMu.RLock()
		model := strings.TrimSpace(s.model)
		effort := strings.TrimSpace(s.effort)
		s.runtimeMu.RUnlock()
		if model != "" {
			params["model"] = model
		}
		if effort != "" {
			params["effort"] = effort
		}
	} else {
		params["model"] = nullableTurnOption(options.Model)
		params["effort"] = nullableTurnOption(options.ReasoningEffort)
		params["serviceTier"] = nullableTurnOption(options.ServiceTier)
	}
	if approval, _ := appServerModeSettings(s.mode); approval != "" {
		params["approvalPolicy"] = approval
	}
	return params
}

func nullableTurnOption(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func (s *appServerSession) storeActiveTurnOptions(options *core.TurnOptions) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	if options == nil {
		s.turnOptions = nil
		return
	}
	copy := *options
	s.turnOptions = &copy
}

// Steer implements core.SteerableSession: it appends user input to the
// in-flight turn via the app-server's native turn/steer method instead of
// starting a new turn. The expectedTurnId precondition makes the server
// reject the request if the snapshotted turn completed or changed in the
// meantime, so a definitive rejection can safely fall back to queueing.
//
// Deliberately NOT done here: mutating currentTurn, clearing pendingMsgs, or
// emitting any lifecycle event — the original turn's single completion/result
// remains authoritative (issue #27).
func (s *appServerSession) Steer(prompt string, images []core.ImageAttachment, files []core.FileAttachment) error {
	if !s.alive.Load() {
		return fmt.Errorf("session is closed: %w", core.ErrSteerRejected)
	}

	if len(files) > 0 {
		filePaths := core.SaveFilesToDisk(s.workDir, files)
		prompt = core.AppendFileRefs(prompt, filePaths)
	}

	prompt, imagePaths, err := s.stageImages(prompt, images)
	if err != nil {
		return fmt.Errorf("%v: %w", err, core.ErrSteerRejected)
	}

	// Snapshot the active turn under the state lock. No preamble handling:
	// a turn in flight implies the preamble decision was already made by the
	// Send that started it.
	s.stateMu.Lock()
	expectedTurn := s.currentTurn
	s.stateMu.Unlock()
	threadID := s.CurrentSessionID()
	if threadID == "" || expectedTurn == "" {
		return core.ErrSteerNoActiveTurn
	}

	input := make([]map[string]any, 0, 1+len(imagePaths))
	input = append(input, map[string]any{
		"type":          "text",
		"text":          prompt,
		"text_elements": []any{},
	})
	for _, path := range imagePaths {
		input = append(input, map[string]any{
			"type": "localImage",
			"path": path,
		})
	}

	params := map[string]any{
		"threadId":       threadID,
		"input":          input,
		"expectedTurnId": expectedTurn,
	}

	// Decode into a raw message so a server response that fails to parse is
	// still recognized as "accepted" rather than misreported as a failure.
	var raw json.RawMessage
	rpcErr, responded := s.requestClassified("turn/steer", params, &raw, appServerRequestTimeout)
	if rpcErr != nil {
		if !responded {
			// Timeout / transport failure: the input may or may not be inside
			// the active turn. Callers must not re-send it automatically.
			return fmt.Errorf("codex app-server turn/steer: %v: %w", rpcErr, core.ErrSteerOutcomeUnknown)
		}
		// Definitive server rejection. If the active turn moved on while the
		// request was in flight, surface that as a turn mismatch.
		s.stateMu.Lock()
		nowTurn := s.currentTurn
		s.stateMu.Unlock()
		if nowTurn != expectedTurn {
			return fmt.Errorf("codex app-server turn/steer: %v: %w", rpcErr, core.ErrSteerTurnMismatch)
		}
		return fmt.Errorf("codex app-server turn/steer: %v: %w", rpcErr, core.ErrSteerRejected)
	}

	var resp turnSteerResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		// The server accepted the steer; only our decode failed. Treat as
		// success (re-sending would duplicate input), but log for diagnosis.
		slog.Warn("codex app-server: turn/steer response decode failed; treating as accepted", "error", err)
		return nil
	}
	if got := resp.turnID(); got != "" && got != expectedTurn {
		// Accepted, but not by the turn we expected. The input WAS delivered
		// somewhere, so this must never trigger a queue fallback.
		return fmt.Errorf("codex app-server turn/steer accepted by turn %s, expected %s: %w", got, expectedTurn, core.ErrSteerOutcomeUnknown)
	}
	return nil
}

func (s *appServerSession) stageImages(prompt string, images []core.ImageAttachment) (string, []string, error) {
	return stageCodexImages(s.workDir, prompt, "codex app-server", images)
}

func (s *appServerSession) RespondPermission(requestID string, result core.PermissionResult) error {
	s.approvalsMu.Lock()
	ch := s.pendingApprovals[requestID]
	s.approvalsMu.Unlock()
	if ch == nil {
		return fmt.Errorf("codex app-server: no pending approval for request %s", requestID)
	}
	select {
	case ch <- result:
	default:
	}
	return nil
}

func (s *appServerSession) handleServerRequest(probe map[string]json.RawMessage) {
	rawID := probe["id"]
	var method string
	if err := json.Unmarshal(probe["method"], &method); err != nil {
		return
	}
	params := probe["params"]
	if appServerRequestIsTurnScoped(method) {
		var route appServerMessageRoute
		if err := json.Unmarshal(params, &route); err == nil && !s.ownsActiveTurn(method, route.ThreadID, route.TurnID) {
			s.rejectUnownedServerRequest(rawID, method)
			return
		}
	}

	switch method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		s.handleApprovalRequest(rawID, method, params)
	case "item/permissions/requestApproval":
		s.handlePermissionsApproval(rawID, params)
	case "item/tool/requestUserInput":
		s.handleRequestUserInput(rawID, params)
	case "item/tool/call":
		s.handleDynamicToolCall(rawID, params)
	default:
		_ = s.writeJSON(map[string]any{
			"jsonrpc": "2.0", "id": rawID,
			"error": map[string]any{"code": -32601, "message": "method not found"},
		})
	}
}

func appServerRequestIsTurnScoped(method string) bool {
	switch method {
	case "item/commandExecution/requestApproval",
		"item/fileChange/requestApproval",
		"item/permissions/requestApproval",
		"item/tool/requestUserInput",
		"item/tool/call":
		return true
	default:
		return false
	}
}

func (s *appServerSession) rejectUnownedServerRequest(rawID json.RawMessage, method string) {
	var result any
	switch method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		result = map[string]any{"decision": "decline"}
	case "item/permissions/requestApproval":
		result = map[string]any{"permissions": map[string]any{}}
	case "item/tool/requestUserInput":
		result = appServerRequestUserInputResponse{Answers: map[string]appServerRequestUserInputAnswer{}}
	case "item/tool/call":
		result = map[string]any{
			"success":      false,
			"contentItems": []map[string]any{{"type": "inputText", "text": "tool not available on this client"}},
		}
	default:
		return
	}
	payload := map[string]any{"jsonrpc": "2.0", "id": rawID, "result": result}
	if err := s.writeJSONWithTimeout(method, payload, appServerResponseWriteTimeout); err != nil {
		slog.Warn("codex app-server: failed to reject request for another turn", "method", method, "error", err)
	}
}

func (s *appServerSession) handleApprovalRequest(rawID json.RawMessage, method string, paramsRaw json.RawMessage) {
	requestID := string(rawID)
	var params map[string]any
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return
	}

	toolName, toolInput := method, appServerJSON(params)
	switch method {
	case "item/commandExecution/requestApproval":
		toolName = "Bash"
		if cmd, _ := params["command"].(string); cmd != "" {
			toolInput = cmd
			if cwd, _ := params["cwd"].(string); cwd != "" {
				toolInput += "\n(in " + cwd + ")"
			}
		}
	case "item/fileChange/requestApproval":
		toolName = "Patch"
		if reason, _ := params["reason"].(string); reason != "" {
			toolInput = reason
		}
	}

	ch := make(chan core.PermissionResult, 1)
	s.approvalsMu.Lock()
	s.pendingApprovals[requestID] = ch
	s.approvalsMu.Unlock()

	s.flushPendingAsThinking()
	s.emit(core.Event{
		Type:         core.EventPermissionRequest,
		RequestID:    requestID,
		ToolName:     toolName,
		ToolInput:    toolInput,
		ToolInputRaw: params,
	})

	go func() {
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		var result core.PermissionResult
		select {
		case result = <-ch:
		case <-s.ctx.Done():
			result = core.PermissionResult{Behavior: "deny"}
		case <-timer.C:
			result = core.PermissionResult{Behavior: "deny"}
		}
		s.approvalsMu.Lock()
		delete(s.pendingApprovals, requestID)
		s.approvalsMu.Unlock()

		decision := "decline"
		if strings.EqualFold(result.Behavior, "allow") {
			decision = "accept"
		}
		_ = s.writeJSON(map[string]any{
			"jsonrpc": "2.0", "id": rawID,
			"result": map[string]any{"decision": decision},
		})
	}()
}

func (s *appServerSession) handlePermissionsApproval(rawID json.RawMessage, paramsRaw json.RawMessage) {
	requestID := string(rawID)
	var params map[string]any
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		return
	}

	ch := make(chan core.PermissionResult, 1)
	s.approvalsMu.Lock()
	s.pendingApprovals[requestID] = ch
	s.approvalsMu.Unlock()

	s.flushPendingAsThinking()
	s.emit(core.Event{
		Type:         core.EventPermissionRequest,
		RequestID:    requestID,
		ToolName:     "Permissions",
		ToolInput:    appServerJSON(params),
		ToolInputRaw: params,
	})

	go func() {
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		var result core.PermissionResult
		select {
		case result = <-ch:
		case <-s.ctx.Done():
			result = core.PermissionResult{Behavior: "deny"}
		case <-timer.C:
			result = core.PermissionResult{Behavior: "deny"}
		}
		s.approvalsMu.Lock()
		delete(s.pendingApprovals, requestID)
		s.approvalsMu.Unlock()

		if strings.EqualFold(result.Behavior, "allow") {
			perms := params["permissions"]
			if perms == nil {
				perms = map[string]any{}
			}
			_ = s.writeJSON(map[string]any{
				"jsonrpc": "2.0", "id": rawID,
				"result": map[string]any{"permissions": perms, "scope": "turn"},
			})
		} else {
			_ = s.writeJSON(map[string]any{
				"jsonrpc": "2.0", "id": rawID,
				"result": map[string]any{"permissions": map[string]any{}},
			})
		}
	}()
}

func (s *appServerSession) handleRequestUserInput(rawID json.RawMessage, paramsRaw json.RawMessage) {
	requestID := string(rawID)
	var params appServerRequestUserInputParams
	if err := json.Unmarshal(paramsRaw, &params); err != nil {
		_ = s.writeJSON(map[string]any{
			"jsonrpc": "2.0", "id": rawID,
			"error": map[string]any{"code": -32602, "message": "invalid params"},
		})
		return
	}

	questions := appServerRequestUserInputQuestions(params.Questions)
	if len(questions) == 0 {
		_ = s.writeJSON(map[string]any{
			"jsonrpc": "2.0", "id": rawID,
			"result": appServerRequestUserInputResponse{Answers: map[string]appServerRequestUserInputAnswer{}},
		})
		return
	}

	rawInput := appServerRequestUserInputRawInput(params)
	ch := make(chan core.PermissionResult, 1)
	s.approvalsMu.Lock()
	s.pendingApprovals[requestID] = ch
	s.approvalsMu.Unlock()

	s.flushPendingAsThinking()
	s.emit(core.Event{
		Type:         core.EventPermissionRequest,
		RequestID:    requestID,
		ToolName:     "AskUserQuestion",
		ToolInput:    appServerJSON(rawInput),
		ToolInputRaw: rawInput,
		Questions:    questions,
	})

	go func() {
		timer := time.NewTimer(5 * time.Minute)
		defer timer.Stop()
		var result core.PermissionResult
		select {
		case result = <-ch:
		case <-s.ctx.Done():
			result = core.PermissionResult{Behavior: "deny"}
		case <-timer.C:
			result = core.PermissionResult{Behavior: "deny"}
		}
		s.approvalsMu.Lock()
		delete(s.pendingApprovals, requestID)
		s.approvalsMu.Unlock()

		response := appServerRequestUserInputResponseFromResult(params.Questions, result)
		_ = s.writeJSON(map[string]any{
			"jsonrpc": "2.0", "id": rawID,
			"result": response,
		})
	}()
}

func (s *appServerSession) handleDynamicToolCall(rawID json.RawMessage, paramsRaw json.RawMessage) {
	_ = s.writeJSON(map[string]any{
		"jsonrpc": "2.0", "id": rawID,
		"result": map[string]any{
			"success":      false,
			"contentItems": []map[string]any{{"type": "inputText", "text": "tool not available on this client"}},
		},
	})
}

func appServerRequestUserInputQuestions(input []appServerRequestUserInputQuestion) []core.UserQuestion {
	questions := make([]core.UserQuestion, 0, len(input))
	for _, in := range input {
		questionText := strings.TrimSpace(in.Question)
		if questionText == "" {
			continue
		}
		q := core.UserQuestion{
			Question: questionText,
			Header:   strings.TrimSpace(in.Header),
		}
		for _, opt := range in.Options {
			q.Options = append(q.Options, core.UserQuestionOption{
				Label:       strings.TrimSpace(opt.Label),
				Description: strings.TrimSpace(opt.Description),
			})
		}
		questions = append(questions, q)
	}
	return questions
}

func appServerRequestUserInputRawInput(params appServerRequestUserInputParams) map[string]any {
	questions := make([]any, 0, len(params.Questions))
	for _, in := range params.Questions {
		q := map[string]any{
			"id":       in.ID,
			"header":   in.Header,
			"question": in.Question,
			"isOther":  in.IsOther,
			"isSecret": in.IsSecret,
			"options":  appServerRequestUserInputRawOptions(in.Options),
		}
		questions = append(questions, q)
	}
	return map[string]any{
		"threadId":  params.ThreadID,
		"turnId":    params.TurnID,
		"itemId":    params.ItemID,
		"questions": questions,
	}
}

func appServerRequestUserInputRawOptions(options []appServerRequestUserInputOption) []any {
	out := make([]any, 0, len(options))
	for _, opt := range options {
		out = append(out, map[string]any{
			"label":       opt.Label,
			"description": opt.Description,
		})
	}
	return out
}

func appServerRequestUserInputResponseFromResult(questions []appServerRequestUserInputQuestion, result core.PermissionResult) appServerRequestUserInputResponse {
	response := appServerRequestUserInputResponse{Answers: map[string]appServerRequestUserInputAnswer{}}
	if !strings.EqualFold(result.Behavior, "allow") {
		return response
	}

	answersRaw, _ := result.UpdatedInput["answers"].(map[string]any)
	if len(answersRaw) == 0 {
		return response
	}

	for _, q := range questions {
		id := strings.TrimSpace(q.ID)
		text := strings.TrimSpace(q.Question)
		if id == "" || text == "" {
			continue
		}
		values := appServerRequestUserInputAnswerValues(answersRaw[text])
		if len(values) == 0 {
			continue
		}
		response.Answers[id] = appServerRequestUserInputAnswer{Answers: values}
	}
	return response
}

func appServerRequestUserInputAnswerValues(raw any) []string {
	switch v := raw.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil
		}
		return []string{v}
	case []string:
		values := make([]string, 0, len(v))
		for _, s := range v {
			if strings.TrimSpace(s) != "" {
				values = append(values, s)
			}
		}
		return values
	case []any:
		values := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				values = append(values, s)
			}
		}
		return values
	case map[string]any:
		return appServerRequestUserInputAnswerValues(v["answers"])
	case appServerRequestUserInputAnswer:
		return appServerRequestUserInputAnswerValues(v.Answers)
	default:
		return nil
	}
}

func (s *appServerSession) rejectPendingApprovals(err error) {
	s.approvalsMu.Lock()
	defer s.approvalsMu.Unlock()
	for id, ch := range s.pendingApprovals {
		delete(s.pendingApprovals, id)
		select {
		case ch <- core.PermissionResult{Behavior: "deny"}:
		default:
		}
	}
}

func (s *appServerSession) Events() <-chan core.Event {
	return s.events
}

func (s *appServerSession) CurrentSessionID() string {
	v, _ := s.threadID.Load().(string)
	return v
}

func (s *appServerSession) GetWorkDir() string {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.workDir
}

func (s *appServerSession) GetModel() string {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	if s.turnOptions != nil {
		return strings.TrimSpace(s.turnOptions.Model)
	}
	return strings.TrimSpace(s.model)
}

func (s *appServerSession) GetReasoningEffort() string {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	if s.turnOptions != nil {
		return strings.TrimSpace(s.turnOptions.ReasoningEffort)
	}
	return strings.TrimSpace(s.effort)
}

func (s *appServerSession) Alive() bool {
	return s.alive.Load()
}

func (s *appServerSession) Close() error {
	s.alive.Store(false)
	s.cancel()

	s.procMu.Lock()
	if s.stdin != nil {
		_ = s.stdin.Close()
		s.stdin = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.procMu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}

	s.closeOnce.Do(func() {
		close(s.events)
	})
	return nil
}

func (s *appServerSession) readLoop(r io.Reader) {
	defer s.wg.Done()
	scanner := bufio.NewScanner(r)
	scanBuf := make([]byte, 0, 64*1024)
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	scanner.Buffer(scanBuf, maxLineSize)

	for scanner.Scan() {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		data := scanner.Bytes()

		var probe map[string]json.RawMessage
		if err := json.Unmarshal(data, &probe); err != nil {
			slog.Debug("codex app-server: invalid JSON", "error", err)
			continue
		}

		_, hasID := probe["id"]
		_, hasMethod := probe["method"]

		switch {
		case hasID && !hasMethod:
			// Response to one of our requests.
			var resp rpcResponseEnvelope
			if err := json.Unmarshal(data, &resp); err != nil {
				slog.Debug("codex app-server: bad response envelope", "error", err)
				continue
			}
			s.handleResponse(resp)

		case hasID && hasMethod:
			// Server-initiated request that requires a response (e.g. approval).
			s.handleServerRequest(probe)

		default:
			// Notification (no id).
			var notif rpcNotificationEnvelope
			if err := json.Unmarshal(data, &notif); err != nil {
				slog.Debug("codex app-server: bad notification envelope", "error", err)
				continue
			}
			s.handleNotification(notif.Method, notif.Params)
		}
	}

	err := scanner.Err()
	if err != nil {
		if s.ctx.Err() == nil && !errors.Is(err, io.EOF) {
			slog.Warn("codex app-server read failed", "error", err)
			if errors.Is(err, bufio.ErrTooLong) {
				s.emitError(fmt.Errorf("codex app-server line exceeds max size (%d bytes): %w", maxLineSize, err))
			} else {
				s.emitError(fmt.Errorf("codex app-server connection closed: %w", err))
			}
		}
		s.alive.Store(false)
		s.rejectPending(err)
		s.rejectPendingApprovals(err)
		return
	}

	s.alive.Store(false)
	s.rejectPending(io.EOF)
	s.rejectPendingApprovals(io.EOF)
}

func (s *appServerSession) stderrLoop(r io.Reader) {
	defer s.wg.Done()
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		slog.Debug("codex app-server stderr", "line", line)
	}
	if err := scanner.Err(); err != nil && s.ctx.Err() == nil {
		slog.Debug("codex app-server stderr read failed", "error", err)
	}
}

func (s *appServerSession) waitLoop() {
	defer s.wg.Done()

	s.procMu.Lock()
	cmd := s.cmd
	s.procMu.Unlock()
	if cmd == nil {
		return
	}

	err := cmd.Wait()
	if s.ctx.Err() == nil && err != nil {
		slog.Warn("codex app-server exited unexpectedly", "error", err)
		s.emitError(fmt.Errorf("codex app-server exited: %w", err))
	}
	s.alive.Store(false)
	if err == nil {
		err = io.EOF
	}
	s.rejectPending(err)
}

func (s *appServerSession) handleResponse(resp rpcResponseEnvelope) {
	id, ok := rpcIDToInt64(resp.ID)
	if !ok {
		return
	}

	s.pendingMu.Lock()
	ch := s.pending[id]
	delete(s.pending, id)
	s.pendingMu.Unlock()

	if ch == nil {
		return
	}

	select {
	case ch <- resp:
	default:
	}
}

func (s *appServerSession) handleNotification(method string, paramsRaw json.RawMessage) {
	switch method {
	case "turn/started":
		var notif turnNotification
		if err := json.Unmarshal(paramsRaw, &notif); err == nil && s.ownsThread(method, notif.ThreadID) {
			s.stateMu.Lock()
			s.currentTurn = notif.Turn.ID
			s.pendingMsgs = s.pendingMsgs[:0]
			s.stateMu.Unlock()
		}

	case "item/started":
		var notif itemNotification
		if err := json.Unmarshal(paramsRaw, &notif); err == nil && s.ownsActiveTurn(method, notif.ThreadID, notif.TurnID) {
			s.handleItemStarted(notif.Item)
		}

	case "item/completed":
		var notif itemNotification
		if err := json.Unmarshal(paramsRaw, &notif); err == nil && s.ownsActiveTurn(method, notif.ThreadID, notif.TurnID) {
			s.handleItemCompleted(notif.Item)
		}

	case "turn/completed":
		var notif turnNotification
		if err := json.Unmarshal(paramsRaw, &notif); err == nil && s.ownsActiveTurn(method, notif.ThreadID, notif.Turn.ID) {
			if notif.Turn.Error != nil && strings.TrimSpace(notif.Turn.Error.Message) != "" {
				s.failTurn(classifyCodexError(fmt.Errorf("%s", notif.Turn.Error.Message)))
				return
			}
			s.completeTurn()
		}

	case "thread/status/changed":
		var notif struct {
			ThreadID string `json:"threadId"`
			Status   struct {
				Type string `json:"type"`
			} `json:"status"`
		}
		if err := json.Unmarshal(paramsRaw, &notif); err == nil && notif.Status.Type == "idle" && s.ownsThread(method, notif.ThreadID) {
			// In codex 0.125+, thread going idle signals turn completion.
			s.completeTurn()
		}

	case "error":
		var notif errorNotification
		if err := json.Unmarshal(paramsRaw, &notif); err == nil && s.ownsActiveTurn(method, notif.ThreadID, notif.TurnID) {
			message := strings.TrimSpace(notif.errorMessage())
			if message != "" && !notif.WillRetry {
				s.emitError(classifyCodexError(fmt.Errorf("%s", message)))
			}
		}
	}
}

// ownsThread applies the app-server routing boundary. Current v2 messages
// always include threadId; accepting an omitted id is an explicit compatibility
// path for older app-server versions that did not expose routing metadata.
func (s *appServerSession) ownsThread(method, threadID string) bool {
	currentThreadID := s.CurrentSessionID()
	if threadID == "" {
		slog.Debug("codex app-server: accepting legacy message without thread id", "method", method)
		return true
	}
	if currentThreadID == "" {
		// During thread/start or thread/resume the response that establishes
		// ownership may race with notifications on the same connection.
		slog.Debug("codex app-server: accepting message before thread ownership is established", "method", method, "thread_id", threadID)
		return true
	}
	if threadID == currentThreadID {
		return true
	}
	slog.Debug("codex app-server: ignoring message for another thread",
		"method", method, "thread_id", threadID, "current_thread_id", currentThreadID)
	return false
}

// ownsActiveTurn accepts only the active root turn once a turnId is present.
// An omitted turnId uses the same explicit legacy compatibility path as an
// omitted threadId; a non-empty id never matches an empty or different turn.
func (s *appServerSession) ownsActiveTurn(method, threadID, turnID string) bool {
	if !s.ownsThread(method, threadID) {
		return false
	}
	if turnID == "" {
		slog.Debug("codex app-server: accepting legacy message without turn id", "method", method)
		return true
	}

	s.stateMu.Lock()
	currentTurn := s.currentTurn
	s.stateMu.Unlock()
	if currentTurn != "" && turnID == currentTurn {
		return true
	}
	slog.Debug("codex app-server: ignoring message for another turn",
		"method", method, "turn_id", turnID, "current_turn_id", currentTurn)
	return false
}

func (s *appServerSession) handleItemStarted(item map[string]any) {
	itemType, _ := item["type"].(string)
	if itemType == "" {
		return
	}

	switch itemType {
	case "agentMessage", "reasoning", "userMessage", "plan", "hookPrompt", "contextCompaction":
		return
	}

	s.flushPendingAsThinking()

	switch itemType {
	case "commandExecution":
		command, _ := item["command"].(string)
		s.emit(core.Event{Type: core.EventToolUse, ToolName: "Bash", ToolInput: command})

	case "mcpToolCall":
		server, _ := item["server"].(string)
		tool, _ := item["tool"].(string)
		name := strings.Trim(strings.Join([]string{server, tool}, ":"), ":")
		s.emit(core.Event{Type: core.EventToolUse, ToolName: "MCP", ToolInput: name + "\n" + appServerJSON(item["arguments"])})

	case "webSearch":
		query, _ := item["query"].(string)
		s.emit(core.Event{Type: core.EventToolUse, ToolName: "WebSearch", ToolInput: query})

	case "dynamicToolCall":
		tool, _ := item["tool"].(string)
		s.emit(core.Event{Type: core.EventToolUse, ToolName: tool, ToolInput: appServerJSON(item["arguments"])})

	case "fileChange":
		s.emit(core.Event{Type: core.EventToolUse, ToolName: "Patch", ToolInput: appServerJSON(item["changes"])})
	}
}

func (s *appServerSession) handleItemCompleted(item map[string]any) {
	itemType, _ := item["type"].(string)
	if itemType == "" {
		return
	}

	switch itemType {
	case "reasoning":
		text := appServerReasoningText(item)
		if text != "" {
			s.emit(core.Event{Type: core.EventThinking, Content: text})
		}

	case "agentMessage":
		text, _ := item["text"].(string)
		if strings.TrimSpace(text) != "" {
			s.stateMu.Lock()
			s.pendingMsgs = append(s.pendingMsgs, text)
			s.stateMu.Unlock()
		}

	case "commandExecution":
		command, _ := item["command"].(string)
		status, _ := item["status"].(string)
		output, _ := item["aggregatedOutput"].(string)
		exitCode, hasExitCode := toInt(item["exitCode"])
		var exitCodePtr *int
		if hasExitCode {
			exitCodePtr = &exitCode
		}
		success := appServerToolSuccess(status, exitCodePtr)
		s.emit(core.Event{
			Type:         core.EventToolResult,
			ToolName:     "Bash",
			ToolInput:    command,
			ToolResult:   truncate(strings.TrimSpace(output), 500),
			ToolStatus:   strings.TrimSpace(status),
			ToolExitCode: exitCodePtr,
			ToolSuccess:  &success,
		})

	case "mcpToolCall":
		tool, _ := item["tool"].(string)
		status, _ := item["status"].(string)
		result := appServerJSON(item["result"])
		if errText := appServerJSON(item["error"]); strings.TrimSpace(errText) != "" && result == "" {
			result = errText
		}
		success := appServerToolSuccess(status, nil)
		s.emit(core.Event{
			Type:        core.EventToolResult,
			ToolName:    tool,
			ToolResult:  truncate(strings.TrimSpace(result), 500),
			ToolStatus:  strings.TrimSpace(status),
			ToolSuccess: &success,
		})

	case "webSearch":
		query, _ := item["query"].(string)
		s.emit(core.Event{
			Type:       core.EventToolResult,
			ToolName:   "WebSearch",
			ToolResult: truncate(strings.TrimSpace(query), 500),
		})

	case "dynamicToolCall":
		tool, _ := item["tool"].(string)
		status, _ := item["status"].(string)
		result := appServerDynamicToolText(item["contentItems"])
		success := appServerToolSuccess(status, nil)
		s.emit(core.Event{
			Type:        core.EventToolResult,
			ToolName:    tool,
			ToolResult:  truncate(strings.TrimSpace(result), 500),
			ToolStatus:  strings.TrimSpace(status),
			ToolSuccess: &success,
		})
	}
}

func appServerReasoningText(item map[string]any) string {
	var parts []string
	if summary, ok := item["summary"].([]any); ok {
		for _, entry := range summary {
			if text, ok := entry.(string); ok && strings.TrimSpace(text) != "" {
				parts = append(parts, text)
			}
		}
	}
	if len(parts) == 0 {
		if content, ok := item["content"].([]any); ok {
			for _, entry := range content {
				if text, ok := entry.(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, text)
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

func appServerDynamicToolText(raw any) string {
	items, ok := raw.([]any)
	if !ok {
		return appServerJSON(raw)
	}
	var parts []string
	for _, entry := range items {
		m, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if text, _ := m["text"].(string); strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return appServerJSON(raw)
	}
	return strings.Join(parts, "\n")
}

func appServerToolSuccess(status string, exitCode *int) bool {
	s := strings.ToLower(strings.TrimSpace(status))
	if exitCode != nil {
		return *exitCode == 0
	}
	return s == "completed" || s == "success" || s == "succeeded" || s == "ok"
}

func normalizeRuntimeReasoningEffort(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "":
		return ""
	case "med":
		return "medium"
	case "x-high", "very-high":
		return "xhigh"
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func appServerJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "{}" || s == "[]" || s == `""` {
		return ""
	}
	return s
}

func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		if err == nil {
			return int(i), true
		}
	}
	return 0, false
}

func rpcIDToInt64(v any) (int64, bool) {
	switch id := v.(type) {
	case float64:
		return int64(id), true
	case int64:
		return id, true
	case int:
		return int64(id), true
	case json.Number:
		i, err := id.Int64()
		return i, err == nil
	}
	return 0, false
}

func (s *appServerSession) completeTurn() {
	s.stateMu.Lock()
	if s.currentTurn == "" {
		s.stateMu.Unlock()
		return
	}
	s.currentTurn = ""
	s.stateMu.Unlock()
	s.flushPendingAsText()
	s.emit(core.Event{Type: core.EventResult, SessionID: s.CurrentSessionID(), Done: true})
}

func (s *appServerSession) failTurn(err error) {
	s.stateMu.Lock()
	s.currentTurn = ""
	s.pendingMsgs = nil
	s.stateMu.Unlock()
	s.emitError(err)
}

func (s *appServerSession) flushPendingAsThinking() {
	s.stateMu.Lock()
	msgs := append([]string(nil), s.pendingMsgs...)
	s.pendingMsgs = s.pendingMsgs[:0]
	s.stateMu.Unlock()

	for _, text := range msgs {
		if strings.TrimSpace(text) != "" {
			s.emit(core.Event{Type: core.EventThinking, Content: text})
		}
	}
}

func (s *appServerSession) flushPendingAsText() {
	s.stateMu.Lock()
	msgs := append([]string(nil), s.pendingMsgs...)
	s.pendingMsgs = s.pendingMsgs[:0]
	s.stateMu.Unlock()

	for _, text := range msgs {
		if strings.TrimSpace(text) != "" {
			s.emit(core.Event{Type: core.EventText, Content: text})
		}
	}
}

func (s *appServerSession) emit(event core.Event) {
	select {
	case s.events <- event:
	default:
		slog.Warn("codex appserver: event channel full, dropping event", "type", event.Type)
	}
}

func (s *appServerSession) emitError(err error) {
	if err == nil {
		return
	}
	s.emit(core.Event{Type: core.EventError, Error: err})
}

func (s *appServerSession) rejectPending(err error) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	for id, ch := range s.pending {
		delete(s.pending, id)
		select {
		case ch <- rpcResponseEnvelope{ID: id, Error: &rpcError{Message: err.Error()}, localFailure: true}:
		default:
		}
	}
}

func (s *appServerSession) request(method string, params any, out any) error {
	return s.requestWithTimeout(method, params, out, appServerRequestTimeout)
}

func (s *appServerSession) requestWithTimeout(method string, params any, out any, timeout time.Duration) error {
	err, _ := s.requestClassified(method, params, out, timeout)
	return err
}

// requestClassified behaves like requestWithTimeout but also reports whether
// the server produced a response envelope for this request. responded=true
// with a non-nil error means the server definitively rejected the request;
// responded=false means the outcome is uncertain (write failure, timeout, or
// session shutdown). Callers that must never duplicate input (turn/steer)
// use this to distinguish "rejected" from "unknown".
func (s *appServerSession) requestClassified(method string, params any, out any, timeout time.Duration) (err error, responded bool) {
	id := s.nextID.Add(1)
	ch := make(chan rpcResponseEnvelope, 1)

	s.pendingMu.Lock()
	if s.pending == nil {
		s.pending = make(map[int64]chan rpcResponseEnvelope)
	}
	s.pending[id] = ch
	s.pendingMu.Unlock()

	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
		"params":  params,
	}

	deadline := time.Now().Add(timeout)
	if err := s.writeJSONWithTimeout(method, payload, timeout); err != nil {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return err, false
	}
	remaining := time.Until(deadline)
	if remaining <= 0 {
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return fmt.Errorf("%s timed out", method), false
	}

	timer := time.NewTimer(remaining)
	defer timer.Stop()
	ctxDone := s.contextDone()
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return classifyCodexError(fmt.Errorf("%s", strings.TrimSpace(resp.Error.Message))), !resp.localFailure
		}
		if out != nil {
			if err := json.Unmarshal(resp.Result, out); err != nil {
				return fmt.Errorf("decode %s response: %w", method, err), true
			}
		}
		return nil, true
	case <-ctxDone:
		return s.contextErr(), false
	case <-timer.C:
		s.pendingMu.Lock()
		delete(s.pending, id)
		s.pendingMu.Unlock()
		return fmt.Errorf("%s timed out", method), false
	}
}

func (s *appServerSession) writeJSONWithTimeout(method string, value any, timeout time.Duration) error {
	done := make(chan error, 1)
	go func() {
		done <- s.writeJSON(value)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case err := <-done:
		return err
	case <-s.contextDone():
		return s.contextErr()
	case <-timer.C:
		err := fmt.Errorf("%s write timed out", method)
		slog.Warn("codex app-server write timed out, closing session", "method", method, "timeout", timeout)
		s.abortTransport()
		return err
	}
}

func (s *appServerSession) contextDone() <-chan struct{} {
	if s.ctx == nil {
		return nil
	}
	return s.ctx.Done()
}

func (s *appServerSession) contextErr() error {
	if s.ctx == nil {
		return context.Canceled
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	return context.Canceled
}

func (s *appServerSession) abortTransport() {
	s.alive.Store(false)
	if s.cancel != nil {
		s.cancel()
	}

	s.procMu.Lock()
	if s.stdin != nil {
		_ = s.stdin.Close()
		s.stdin = nil
	}
	if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.procMu.Unlock()
}

func (s *appServerSession) notify(method string, params any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		payload["params"] = params
	}
	return s.writeJSON(payload)
}

func (s *appServerSession) writeJSON(v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("codex app-server encode: %w", err)
	}

	s.procMu.Lock()
	stdin := s.stdin
	s.procMu.Unlock()
	if stdin == nil {
		return fmt.Errorf("codex app-server connection is closed")
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := stdin.Write(append(b, '\n')); err != nil {
		return fmt.Errorf("codex app-server write: %w", err)
	}
	return nil
}

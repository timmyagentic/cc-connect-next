package core

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func readBridgeHealth(t *testing.T, e *Engine) (RuntimeHealth, int) {
	t.Helper()
	api := &APIServer{engines: map[string]*Engine{e.name: e}}
	rec := httptest.NewRecorder()
	api.handleHealth(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var health RuntimeHealth
	if err := json.Unmarshal(rec.Body.Bytes(), &health); err != nil {
		t.Fatal(err)
	}
	return health, rec.Code
}

func TestBridgeStandbyDoesNotBlockNativeReadiness(t *testing.T) {
	for _, tt := range []struct {
		name      string
		native    bool
		ready     bool
		failed    bool
		wantReady bool
	}{
		{name: "native ready", native: true, ready: true, wantReady: true},
		{name: "native starting", native: true},
		{name: "native disconnected", native: true, ready: true, failed: true},
		{name: "bridge only"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var platforms []Platform
			p := &stubUnhealthyPlatform{stubPlatformEngine: stubPlatformEngine{n: "native"}}
			if tt.failed {
				p.err = errors.New("connection lost")
			}
			if tt.native {
				platforms = append(platforms, p)
			}
			e := NewEngine("test", &stubAgent{}, platforms, "", LangEnglish)
			if tt.ready {
				e.OnPlatformReady(p)
			}
			bs := NewBridgeServerInsecure(0, "", "/bridge/ws", nil)
			bp := bs.NewPlatform("test")
			bs.RegisterEngine("test", e, bp)
			e.AddPlatform(bp)

			health, code := readBridgeHealth(t, e)
			bridge := health.Projects[0].Platforms[len(platforms)]
			if health.Ready != tt.wantReady || (code == http.StatusOK) != tt.wantReady || bridge.State != "standby" {
				t.Fatalf("health=%+v HTTP=%d, want ready=%t and Bridge standby", health, code, tt.wantReady)
			}
			if got := e.updates().connectionsReady(); got != tt.wantReady {
				t.Fatalf("update receipt connectionsReady=%t, want %t", got, tt.wantReady)
			}
			status := e.PlatformStatuses()[len(platforms)]
			manifest := e.platformRuntimeCapabilities(bp, status, capabilitySnapshot{})
			if manifest.State != CapabilityConditional || status.Usable() {
				t.Fatalf("standby capability=%+v usable=%t, want conditional and unusable", manifest, status.Usable())
			}
			for _, capability := range manifest.Capabilities {
				switch capability.ID {
				case "recent_conversation_context", "direct_user_messages", "completion_notification":
					if capability.Availability.State != CapabilityUnavailable {
						t.Fatalf("unsupported Bridge capability %s = %+v", capability.ID, capability.Availability)
					}
				}
			}
		})
	}
}

func TestBridgeReadinessTracksAdapterDisconnectAndReconnect(t *testing.T) {
	bs, wsURL := startTestBridge(t, "")
	bp := bs.NewPlatform("test")
	e := NewEngine("test", &stubAgent{}, nil, "", LangEnglish)
	bs.RegisterEngine("test", e, bp)
	e.AddPlatform(bp)

	health, code := readBridgeHealth(t, e)
	if code != http.StatusServiceUnavailable || health.Projects[0].Platforms[0].State != "standby" {
		t.Fatalf("before first adapter: HTTP=%d health=%+v", code, health)
	}
	connect := func() *websocket.Conn {
		t.Helper()
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		register(t, conn, "health-test", []string{"text"})
		waitForBridgeStatus(t, e, func(s PlatformStatus) bool { return s.Usable() })
		health, code := readBridgeHealth(t, e)
		if code != http.StatusOK || !health.Ready || !e.updates().connectionsReady() {
			t.Fatalf("connected adapter: HTTP=%d health=%+v", code, health)
		}
		return conn
	}
	conn := connect()
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	waitForBridgeStatus(t, e, func(s PlatformStatus) bool { return !s.Usable() && s.Err != nil })
	health, code = readBridgeHealth(t, e)
	if code != http.StatusServiceUnavailable || health.Projects[0].Platforms[0].State != RuntimePlatformUnavailable || e.updates().connectionsReady() {
		t.Fatalf("disconnected adapter: HTTP=%d health=%+v", code, health)
	}
	connect()
}

func TestBridgeListenerFailureIsUnavailableWithReadyNativePlatform(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	bs := NewBridgeServerInsecure(listener.Addr().(*net.TCPAddr).Port, "", "/bridge/ws", nil)
	native := &stubPlatformEngine{n: "native"}
	e := NewEngine("test", &stubAgent{}, []Platform{native}, "", LangEnglish)
	e.OnPlatformReady(native)
	bp := bs.NewPlatform("test")
	bs.RegisterEngine("test", e, bp)
	e.AddPlatform(bp)
	bs.Start()
	t.Cleanup(func() { _ = bs.server.Close() })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var listenErr *net.OpError
		if errors.As(bp.ConnectionError(), &listenErr) && listenErr.Op == "listen" {
			health, code := readBridgeHealth(t, e)
			bridge := health.Projects[0].Platforms[1]
			if bridge.State != RuntimePlatformUnavailable || health.Ready || code != http.StatusServiceUnavailable {
				t.Fatalf("listener failure reported ready: HTTP=%d health=%+v", code, health)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("listener error was not reported: %v", bp.ConnectionError())
}

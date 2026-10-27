package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

type groupModelManifestAgent struct {
	stubModelModeAgent
	supported bool
}

func (a *groupModelManifestAgent) SupportsPerTurnModel() bool { return a.supported }

func TestGroupModelCapability_SessionProviderIdentityDoesNotExposeConfig(t *testing.T) {
	e := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	for _, session := range []AgentSession{&groupProviderSession{provider: "private-provider"}, &cronModelSession{}} {
		capability := findRuntimeFeature(t, e.sessionRuntimeCapabilities(session).Capabilities, "model_provider_identity")
		_, supported := session.(ModelProviderSession)
		if (capability.Availability.State == CapabilityAvailable) != supported || capability.ReadOnly == nil || !*capability.ReadOnly || capability.Fallback.Mode != "reject" {
			t.Fatalf("provider identity contract is inaccurate: %+v", capability)
		}
		if strings.Contains(fmt.Sprint(capability), "private-provider") {
			t.Fatal("capability manifest exposed a configuration value")
		}
	}
}

func TestGroupModelCapability_LegacySwitchingRemainsAvailable(t *testing.T) {
	for _, tc := range []struct {
		name       string
		agent      Agent
		want       CapabilityAvailabilityState
		groupReady bool
	}{
		{"no-model-interface", &stubAgent{}, CapabilityUnavailable, false},
		{"legacy-switcher-only", &stubModelModeAgent{}, CapabilityAvailable, false},
		{"per-turn-opt-out", &groupModelManifestAgent{}, CapabilityAvailable, false},
		{"non-codex-per-turn-support", &groupModelManifestAgent{supported: true}, CapabilityAvailable, true},
		{"per-turn-without-switcher", &manifestPerTurnModelAgent{supported: true}, CapabilityUnavailable, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := NewEngine("demo", tc.agent, nil, "", LangEnglish)
			command := findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "model")
			if command.Availability.State != tc.want {
				t.Fatalf("availability = %#v, want %s", command.Availability, tc.want)
			}
			if command.Availability.Reason == "" || command.Availability.ReasonZH == "" {
				t.Fatalf("missing bilingual availability reason: %#v", command.Availability)
			}
			if tc.want != CapabilityAvailable {
				return
			}
			if command.Permission != CapabilityPermissionMember || len(command.PrivilegedWhen) != 0 {
				t.Fatalf("group models changed the legacy member policy: %#v", command)
			}
			if !strings.Contains(command.Availability.Reason, "legacy project/workspace default switching") {
				t.Fatalf("legacy switching not explained: %s", command.Availability.Reason)
			}
			if tc.groupReady {
				for _, condition := range []string{"verified chat identity", "persistent storage", "current model list", "concrete session support"} {
					if !strings.Contains(command.Availability.Reason, condition) {
						t.Errorf("group availability omits %q: %s", condition, command.Availability.Reason)
					}
				}
			} else if !strings.Contains(command.Availability.Reason, "Chat pinning is unavailable") {
				t.Fatalf("unsupported group pinning not distinguished from legacy switching: %s", command.Availability.Reason)
			}
		})
	}
}

func TestGroupModelCapability_ScopePersistenceAndPrecedence(t *testing.T) {
	e := NewEngine("demo", &groupModelManifestAgent{supported: true}, nil, "", LangEnglish)
	command := findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "model")
	if command.Usage != "/model [switch <name>|chat [<name>|reset]]" {
		t.Fatalf("usage = %q", command.Usage)
	}
	if command.ReadOnly || !hasManifestEffect(command.SideEffects, "persistent_state") || !hasManifestEffect(command.SideEffects, "configuration") || !hasManifestEffect(command.SideEffects, "agent_process") {
		t.Fatalf("missing group persistence or legacy effects: %#v", command)
	}
	parameters := make(map[string]CapabilityParameter)
	for _, parameter := range command.Parameters {
		parameters[parameter.Name] = parameter
		if parameter.Required || parameter.Type != "string" || parameter.DescriptionZH == "" {
			t.Fatalf("incomplete optional model parameter: %#v", parameter)
		}
	}
	if !slices.Equal(parameters["action"].AllowedValues, []string{"switch", "chat", "group"}) {
		t.Fatalf("model actions = %v", parameters["action"].AllowedValues)
	}
	for name, fragments := range map[string][]string{
		"action": {"verified private or group chat", "group remains an alias", "DMs are isolated by user and bot/project", "all members", "existing/new topics", "ModelSwitcher", "PerTurnModelSupporter", "no Provider change", "legacy /model <name>"},
		"name":   {"current model list", "reset", "within the project", "platform + stable group ID", "platform:dm:userID", "not topic or workspace", "/reset", "/new", "restarts", "execution on all three send paths", "queued requests", "next new turn", "without interrupting", "explicit one-shot /fast or task model takes precedence"},
	} {
		for _, fragment := range fragments {
			if !strings.Contains(parameters[name].Description, fragment) {
				t.Errorf("%s parameter omits %q: %s", name, fragment, parameters[name].Description)
			}
		}
	}
	if command.Fallback.Mode != "reject" || command.Fallback.DescriptionZH == "" {
		t.Fatalf("group binding must fail explicitly: %#v", command.Fallback)
	}
	for _, fragment := range []string{"unsupported per-turn", "backend/Provider mismatch", "model/application errors", "without silently falling back", "reselect", "reset"} {
		if !strings.Contains(command.Fallback.Description, fragment) {
			t.Errorf("fallback omits %q: %s", fragment, command.Fallback.Description)
		}
	}
	for _, args := range [][]string{nil, {"switch", "example"}, {"chat"}, {"chat", "example"}, {"chat", "reset"}, {"group"}, {"group", "example"}, {"group", "reset"}} {
		if isPrivilegedCommandInvocation("model", args) {
			t.Errorf("/model %v unexpectedly requires admin", args)
		}
	}
}

func TestGroupModelI18n_AllLanguagesAndFormatArguments(t *testing.T) {
	for _, tc := range []struct {
		key  MsgKey
		args []any
	}{
		{MsgModelGroupOnly, nil},
		{MsgModelGroupStorageUnavailable, nil},
		{MsgModelGroupUnsupported, nil},
		{MsgModelGroupProviderChanged, nil},
		{MsgModelGroupSessionProviderChanged, nil},
		{MsgModelGroupUnknown, []any{"requested-model"}},
		{MsgModelGroupUsage, nil},
		{MsgModelGroupReset, nil},
		{MsgModelGroupChanged, []any{"selected-model"}},
		{MsgModelGroupCurrent, []any{"selected-model"}},
		{MsgModelGroupDefault, nil},
		{MsgModelGroupResetButton, nil},
		{MsgModelGroupSelect, nil},
		{MsgModelScopeProject, nil},
		{MsgModelScopeWorkspace, nil},
		{MsgModelChanged, []any{"selected-model"}},
		{MsgModelCardSwitching, []any{"selected-model"}},
		{MsgModelCardSwitched, []any{"selected-model"}},
		{MsgModelUsage, nil},
		{MsgModelSelectPlaceholder, nil},
	} {
		for _, language := range []Language{LangEnglish, LangChinese, LangTraditionalChinese, LangJapanese, LangSpanish} {
			t.Run(string(tc.key)+"/"+string(language), func(t *testing.T) {
				translation, ok := messages[tc.key][language]
				if !ok || strings.TrimSpace(translation) == "" {
					t.Fatal("missing explicit translation; English fallback is insufficient")
				}
				formatted := fmt.Sprintf(translation, tc.args...)
				if strings.Contains(formatted, "%!") {
					t.Fatalf("format argument contract changed: %s", formatted)
				}
				for _, arg := range tc.args {
					if !strings.Contains(formatted, arg.(string)) {
						t.Fatalf("missing argument %q: %s", arg, formatted)
					}
				}
			})
		}
	}
}

func TestGroupModelI18n_LegacyDefaultScopeIsExplicit(t *testing.T) {
	for _, tc := range []struct {
		language  Language
		project   string
		workspace string
		fixed     string
	}{
		{LangEnglish, "Project", "workspace", "fixed"},
		{LangChinese, "项目", "工作区", "固定"},
		{LangTraditionalChinese, "專案", "工作區", "固定"},
		{LangJapanese, "プロジェクト", "ワークスペース", "固定"},
		{LangSpanish, "proyecto", "espacio", "fijo"},
	} {
		for _, key := range []MsgKey{MsgModelChanged, MsgModelCardSwitching, MsgModelCardSwitched, MsgModelUsage, MsgModelSelectPlaceholder} {
			text := strings.ToLower(messages[key][tc.language])
			for _, fragment := range []string{tc.project, tc.workspace, tc.fixed} {
				if !strings.Contains(text, strings.ToLower(fragment)) {
					t.Errorf("%s/%s omits default/fixed-group scope %q: %s", key, tc.language, fragment, text)
				}
			}
		}
	}
}

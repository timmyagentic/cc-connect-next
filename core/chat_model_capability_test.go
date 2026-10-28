package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

type chatModelManifestAgent struct {
	stubModelModeAgent
	supported bool
}

func (a *chatModelManifestAgent) SupportsPerTurnModel() bool { return a.supported }

type chatModelProviderIdentitySession struct {
	cronModelSession
}

func (*chatModelProviderIdentitySession) ModelProviderName() string { return "private-provider" }

func TestChatModelCapability_SessionProviderIdentityDoesNotExposeConfig(t *testing.T) {
	e := NewEngine("demo", &stubAgent{}, nil, "", LangEnglish)
	for _, session := range []AgentSession{&chatModelProviderIdentitySession{}, &cronModelSession{}} {
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

func TestChatModelCapability_DefaultAvailabilityAndChatPrerequisites(t *testing.T) {
	for _, tc := range []struct {
		name      string
		agent     Agent
		want      CapabilityAvailabilityState
		chatReady bool
	}{
		{"no-model-interface", &stubAgent{}, CapabilityUnavailable, false},
		{"default-only-switcher", &stubModelModeAgent{}, CapabilityAvailable, false},
		{"per-turn-opt-out", &chatModelManifestAgent{}, CapabilityAvailable, false},
		{"non-codex-per-turn-support", &chatModelManifestAgent{supported: true}, CapabilityAvailable, true},
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
			if command.Permission != CapabilityPermissionConditional || !slices.Equal(command.PrivilegedWhen, []string{"default"}) {
				t.Fatalf("only global-default mutations may require admin: %#v", command)
			}
			if tc.want != CapabilityAvailable {
				return
			}
			for _, condition := range []string{"ModelSwitcher supports /model default", "current bot/project", "Queries and current-chat changes are member operations", "global-default changes require admin"} {
				if !strings.Contains(command.Availability.Reason, condition) {
					t.Errorf("availability omits %q: %s", condition, command.Availability.Reason)
				}
			}
			if tc.chatReady {
				for _, condition := range []string{"verified chat identity", "persistent storage", "current model list", "concrete session support"} {
					if !strings.Contains(command.Availability.Reason, condition) {
						t.Errorf("chat availability omits %q: %s", condition, command.Availability.Reason)
					}
				}
			} else {
				for _, condition := range []string{"Chat pinning is unavailable", "/model reset", "verified chat identity and storage"} {
					if !strings.Contains(command.Availability.Reason, condition) {
						t.Errorf("unsupported chat pinning/recovery omits %q: %s", condition, command.Availability.Reason)
					}
				}
			}
		})
	}
}

func TestChatModelCapability_ScopePersistenceAndPrecedence(t *testing.T) {
	e := NewEngine("demo", &chatModelManifestAgent{supported: true}, nil, "", LangEnglish)
	command := findManifestCommand(t, e.QueryAgentCapabilityManifest("", "", false).Commands, "model")
	if command.Usage != "/model [<name|alias|number>|reset|default [<name|alias|number>]]" {
		t.Fatalf("usage = %q", command.Usage)
	}
	definition, ok := builtinCommandByID("model")
	if !ok || !slices.Equal(definition.subcommands, []string{"default"}) || !slices.Equal(definition.privilegedWhen, []string{"default"}) {
		t.Fatalf("model must expose only the default subcommand: %#v", definition)
	}
	if command.ReadOnly || !hasManifestEffect(command.SideEffects, "persistent_state") || !hasManifestEffect(command.SideEffects, "configuration") || !hasManifestEffect(command.SideEffects, "agent_process") {
		t.Fatalf("missing chat persistence or global-default effects: %#v", command)
	}
	parameters := make(map[string]CapabilityParameter)
	for _, parameter := range command.Parameters {
		parameters[parameter.Name] = parameter
		if parameter.Required || parameter.Type != "string" || parameter.DescriptionZH == "" {
			t.Fatalf("incomplete optional model parameter: %#v", parameter)
		}
	}
	if !slices.Equal(parameters["action"].AllowedValues, []string{"default"}) {
		t.Fatalf("model actions = %v", parameters["action"].AllowedValues)
	}
	for name, fragments := range map[string][]string{
		"action": {"verified private or group chat", "current bot/project only, not other bots", "DMs are isolated by user and bot/project", "all members", "existing/new topics", "ModelSwitcher", "PerTurnModelSupporter", "no Provider change", "only /model default <name|alias|number> requires admin"},
		"name":   {"current model list", "/model reset", "within the project", "platform + stable group ID", "platform:dm:userID", "not topic or workspace", "/reset", "/new", "restarts", "execution on all three send paths", "queued requests", "next new turn", "without interrupting", "explicit one-shot /fast or task model takes precedence", "read-only query", "otherwise the current bot/project's global default", "only chats without a fixed model in the current bot/project"},
	} {
		for _, fragment := range fragments {
			if !strings.Contains(parameters[name].Description, fragment) {
				t.Errorf("%s parameter omits %q: %s", name, fragment, parameters[name].Description)
			}
		}
	}
	if command.Fallback.Mode != "reject" || command.Fallback.DescriptionZH == "" {
		t.Fatalf("chat binding must fail explicitly: %#v", command.Fallback)
	}
	for _, fragment := range []string{"unsupported per-turn", "backend/Provider mismatch", "model/application errors", "without silently falling back", "reselect with /model <name>", "/model reset", "stale card actions", "without changing settings"} {
		if !strings.Contains(command.Fallback.Description, fragment) {
			t.Errorf("fallback omits %q: %s", fragment, command.Fallback.Description)
		}
	}
	for _, removed := range []string{"/model chat", "/model group", "/model switch", "workspace default", "workspace/project default"} {
		if strings.Contains(fmt.Sprint(command), removed) {
			t.Errorf("manifest still advertises removed model syntax/scope %q", removed)
		}
	}
}

func TestChatModelCapability_PermissionsMatchInvocation(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		wantAdmin bool
	}{
		{"chat-query", nil, false},
		{"chat-model", []string{"example"}, false},
		{"chat-number", []string{"1"}, false},
		{"chat-reset", []string{"reset"}, false},
		{"default-query", []string{"default"}, false},
		{"default-query-case-insensitive", []string{"DEFAULT"}, false},
		{"default-model", []string{"default", "example"}, true},
		{"default-number", []string{"default", "1"}, true},
		{"default-model-case-insensitive", []string{"DEFAULT", "example"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPrivilegedCommandInvocation("model", tc.args); got != tc.wantAdmin {
				t.Fatalf("/model %v requires admin = %t, want %t", tc.args, got, tc.wantAdmin)
			}
		})
	}
}

func TestChatModelI18n_AllLanguagesAndFormatArguments(t *testing.T) {
	for _, tc := range []struct {
		key  MsgKey
		args []any
	}{
		{MsgModelChatOnly, nil},
		{MsgModelChatStorageUnavailable, nil},
		{MsgModelChatUnsupported, nil},
		{MsgModelChatProviderChanged, nil},
		{MsgModelChatSessionProviderChanged, nil},
		{MsgModelChatUnknown, []any{"requested-model"}},
		{MsgModelChatUsage, nil},
		{MsgModelChatReset, nil},
		{MsgModelChatChanged, []any{"selected-model"}},
		{MsgModelChatCurrent, []any{"selected-model"}},
		{MsgModelChatDefault, []any{"inherited-model"}},
		{MsgModelChatResetButton, nil},
		{MsgModelChatSelect, nil},
		{MsgModelScopeProject, nil},
		{MsgModelDefaultUsage, nil},
		{MsgModelDefaultButton, nil},
		{MsgModelDefaultTitle, nil},
		{MsgModelDefaultChanged, []any{"selected-model"}},
		{MsgModelCurrent, []any{"selected-model"}},
		{MsgModelDefault, nil},
		{MsgModelUsage, nil},
		{MsgModelSelectPlaceholder, nil},
		{MsgBuiltinCmdModel, nil},
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

func TestChatModelI18n_ChatAndGlobalScopesAreDistinct(t *testing.T) {
	for _, tc := range []struct {
		language Language
		bot      string
		project  string
		fixed    string
		admin    string
	}{
		{LangEnglish, "bot", "project", "fixed", "admin"},
		{LangChinese, "机器人", "项目", "固定", "管理员"},
		{LangTraditionalChinese, "機器人", "專案", "固定", "管理員"},
		{LangJapanese, "ボット", "プロジェクト", "固定", "管理者"},
		{LangSpanish, "bot", "proyecto", "fijo", "administrador"},
	} {
		t.Run(string(tc.language), func(t *testing.T) {
			for _, key := range []MsgKey{MsgModelScopeProject, MsgModelDefaultUsage, MsgModelDefaultTitle, MsgModelDefaultChanged, MsgModelSelectPlaceholder} {
				text := strings.ToLower(messages[key][tc.language])
				for _, fragment := range []string{tc.bot, tc.project} {
					if !strings.Contains(text, fragment) {
						t.Errorf("%s omits global scope %q: %s", key, fragment, text)
					}
				}
			}
			if !strings.Contains(messages[MsgModelDefaultUsage][tc.language], tc.admin) {
				t.Error("global-default usage omits the administrator requirement")
			}
			if !strings.Contains(messages[MsgModelChatCurrent][tc.language], tc.fixed) {
				t.Error("current-chat model does not identify a fixed selection")
			}
			for _, key := range []MsgKey{MsgModelChatCurrent, MsgModelChatDefault, MsgModelDefaultChanged} {
				if strings.Count(messages[key][tc.language], "%s") != 1 {
					t.Errorf("%s must show exactly one actual model", key)
				}
			}
			if !strings.Contains(messages[MsgModelChatUsage][tc.language], "/model [<name|alias|number>|reset]") || !strings.Contains(messages[MsgModelDefaultUsage][tc.language], "/model default [<name|alias|number>]") {
				t.Error("model scope usage is incomplete")
			}
			for _, key := range []MsgKey{MsgHelp, MsgHelpAgentSection} {
				text := messages[key][tc.language]
				if !strings.Contains(text, "/model [<name|alias|number>|reset]") || !strings.Contains(text, "/model default [<name|alias|number>]") {
					t.Errorf("%s does not describe both model scopes", key)
				}
			}
		})
	}
	for key, translations := range messages {
		for language, text := range translations {
			for _, removed := range []string{"/model chat", "/model group", "/model switch", "/model [switch"} {
				if strings.Contains(text, removed) {
					t.Errorf("%s/%s still advertises removed command %q", key, language, removed)
				}
			}
		}
	}
}

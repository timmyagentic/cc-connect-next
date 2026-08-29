package core

import (
	"fmt"
	"sort"
	"strings"
)

func SearchAgentCapabilityManifest(manifest AgentCapabilityManifest, query string) AgentCapabilityManifest {
	query = strings.TrimSpace(query)
	if query == "" {
		return manifest
	}
	result := manifest
	result.Query = query
	result.Configuration = SearchConfigCatalog(manifest.Configuration, query)
	result.Tools = filterAgentTools(manifest.Tools, query)
	result.Commands = filterAgentCommands(manifest.Commands, query)
	result.Skills = filterAgentSkills(manifest.Skills, query)
	result.Runtime = filterRuntimeAdapters(manifest.Runtime, query)
	return result
}

func AgentCapabilityManifestHasMatches(manifest AgentCapabilityManifest) bool {
	return len(manifest.Configuration.Capabilities) > 0 || len(manifest.Configuration.Options) > 0 ||
		len(manifest.Tools) > 0 || len(manifest.Commands) > 0 || len(manifest.Skills) > 0 || len(manifest.Runtime) > 0
}

func SelectAgentCapabilityManifestSections(manifest AgentCapabilityManifest, raw string) AgentCapabilityManifest {
	if strings.TrimSpace(raw) == "" {
		return manifest
	}
	selected := make(map[string]bool)
	for _, part := range strings.Split(raw, ",") {
		if value := strings.ToLower(strings.TrimSpace(part)); value != "" {
			selected[value] = true
		}
	}
	if !selected["configuration"] {
		manifest.Configuration = ConfigCatalog{Version: manifest.Configuration.Version}
	}
	if !selected["tools"] {
		manifest.Tools = nil
	}
	if !selected["commands"] {
		manifest.Commands = nil
	}
	if !selected["skills"] {
		manifest.Skills = nil
	}
	if !selected["runtime"] {
		manifest.Runtime = nil
	}
	return manifest
}

func capabilityParameterValues(parameters []CapabilityParameter) []string {
	var result []string
	for _, parameter := range parameters {
		result = append(result, parameter.Name, parameter.Type, parameter.Description, parameter.DescriptionZH)
		result = append(result, parameter.AllowedValues...)
	}
	return result
}

func capabilityEffectValues(effects []CapabilitySideEffect) []string {
	var result []string
	for _, effect := range effects {
		result = append(result, effect.Kind, effect.Description, effect.DescriptionZH)
	}
	return result
}

func filterAgentTools(values []AgentToolCapability, query string) []AgentToolCapability {
	type scored struct {
		value AgentToolCapability
		score int
	}
	var matches []scored
	for _, value := range values {
		search := []string{value.ID, value.Invocation, value.Description, value.DescriptionZH, string(value.Permission), string(value.Availability.State), value.Availability.Reason, value.Availability.ReasonZH, value.Fallback.Mode, value.Fallback.Description, value.Fallback.DescriptionZH}
		search = append(search, capabilityParameterValues(value.Parameters)...)
		search = append(search, capabilityEffectValues(value.SideEffects)...)
		if score := catalogTextMatchScore(strings.ToLower(query), search...); score > 0 {
			matches = append(matches, scored{value: value, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	result := make([]AgentToolCapability, 0, len(matches))
	for _, match := range matches {
		result = append(result, match.value)
	}
	return result
}

func filterAgentCommands(values []AgentCommandCapability, query string) []AgentCommandCapability {
	type scored struct {
		value AgentCommandCapability
		score int
	}
	var matches []scored
	for _, value := range values {
		search := []string{value.ID, value.Invocation, value.Source, value.Category, value.Usage, value.Description, value.DescriptionZH, string(value.Permission), string(value.Availability.State), value.Availability.Reason, value.Availability.ReasonZH, value.Fallback.Mode, value.Fallback.Description, value.Fallback.DescriptionZH}
		search = append(search, value.Aliases...)
		search = append(search, value.PrivilegedWhen...)
		search = append(search, capabilityParameterValues(value.Parameters)...)
		search = append(search, capabilityEffectValues(value.SideEffects)...)
		if score := catalogTextMatchScore(strings.ToLower(query), search...); score > 0 {
			matches = append(matches, scored{value: value, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	result := make([]AgentCommandCapability, 0, len(matches))
	for _, match := range matches {
		result = append(result, match.value)
	}
	return result
}

func filterAgentSkills(values []AgentSkillCapability, query string) []AgentSkillCapability {
	type scored struct {
		value AgentSkillCapability
		score int
	}
	var matches []scored
	for _, value := range values {
		search := []string{value.Name, value.DisplayName, value.Invocation, value.Description, string(value.Permission), string(value.Availability.State), value.Availability.Reason, value.Availability.ReasonZH}
		search = append(search, capabilityParameterValues(value.Parameters)...)
		search = append(search, capabilityEffectValues(value.SideEffects)...)
		if score := catalogTextMatchScore(strings.ToLower(query), search...); score > 0 {
			matches = append(matches, scored{value: value, score: score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	result := make([]AgentSkillCapability, 0, len(matches))
	for _, match := range matches {
		result = append(result, match.value)
	}
	return result
}

func filterRuntimeAdapters(values []RuntimeAdapterCapabilities, query string) []RuntimeAdapterCapabilities {
	var result []RuntimeAdapterCapabilities
	lower := strings.ToLower(query)
	for _, adapter := range values {
		adapterMatch := catalogTextMatchScore(lower, adapter.Kind, adapter.Name, string(adapter.State), adapter.Reason) > 0
		filtered := adapter
		filtered.Capabilities = nil
		for _, capability := range adapter.Capabilities {
			if adapterMatch || catalogTextMatchScore(lower,
				capability.ID, capability.Description, capability.DescriptionZH,
				string(capability.Availability.State), capability.Availability.Reason, capability.Availability.ReasonZH,
				capability.Fallback.Mode, capability.Fallback.Description, capability.Fallback.DescriptionZH,
			) > 0 {
				filtered.Capabilities = append(filtered.Capabilities, capability)
			}
		}
		if len(filtered.Capabilities) > 0 {
			result = append(result, filtered)
		}
	}
	return result
}

func RenderAgentCapabilityManifestMarkdown(manifest AgentCapabilityManifest, language string) string {
	zh := strings.HasPrefix(strings.ToLower(strings.TrimSpace(language)), "zh")
	var b strings.Builder
	if zh {
		fmt.Fprintf(&b, "# cc-connect-next Agent 能力清单\n\n项目：`%s` · 版本：`%s` · Schema：`%s`\n\n", manifest.Project, manifest.Version, manifest.Schema)
		b.WriteString("本清单是只读能力元数据，不包含当前配置值、凭证、Skill 正文、自定义 Prompt/Exec 正文或 Shell 命令正文。\n")
	} else {
		fmt.Fprintf(&b, "# cc-connect-next Agent Capability Manifest\n\nProject: `%s` · version: `%s` · schema: `%s`\n\n", manifest.Project, manifest.Version, manifest.Schema)
		b.WriteString("This is read-only capability metadata. It contains no current config values, credentials, Skill bodies, custom Prompt/Exec bodies, or shell command bodies.\n")
	}
	if manifest.Query != "" {
		if zh {
			fmt.Fprintf(&b, "\n查询：`%s`\n", manifest.Query)
		} else {
			fmt.Fprintf(&b, "\nQuery: `%s`\n", manifest.Query)
		}
	}
	if zh {
		fmt.Fprintf(&b, "\n活动 Agent：`%s` · 活动 Platform：`%s` · 已编译：%d Agent / %d Platform\n", manifest.ActiveAgent, strings.Join(manifest.ActivePlatforms, ", "), len(manifest.CompiledAgents), len(manifest.CompiledPlatforms))
	} else {
		fmt.Fprintf(&b, "\nActive Agent: `%s` · active Platforms: `%s` · compiled: %d Agents / %d Platforms\n", manifest.ActiveAgent, strings.Join(manifest.ActivePlatforms, ", "), len(manifest.CompiledAgents), len(manifest.CompiledPlatforms))
	}

	renderToolCapabilities(&b, manifest.Tools, zh)
	renderCommandCapabilities(&b, manifest.Commands, zh)
	renderSkillCapabilities(&b, manifest.Skills, zh)
	renderRuntimeCapabilities(&b, manifest.Runtime, zh)
	renderManifestConfiguration(&b, manifest, zh)
	return strings.TrimSpace(b.String())
}

func localized(zh bool, en, chinese string) string {
	if zh && chinese != "" {
		return chinese
	}
	return en
}

func renderToolCapabilities(b *strings.Builder, tools []AgentToolCapability, zh bool) {
	if len(tools) == 0 {
		return
	}
	b.WriteString(map[bool]string{true: "\n\n## Agent CLI 工具\n", false: "\n\n## Agent CLI tools\n"}[zh])
	for _, tool := range tools {
		fmt.Fprintf(b, "\n### `%s`\n\n%s\n", tool.Invocation, localized(zh, tool.Description, tool.DescriptionZH))
		renderActionContract(b, tool.Permission, tool.ReadOnly, tool.SideEffects, tool.Fallback, tool.Availability, tool.Parameters, zh)
	}
}

func renderCommandCapabilities(b *strings.Builder, commands []AgentCommandCapability, zh bool) {
	if len(commands) == 0 {
		return
	}
	b.WriteString(map[bool]string{true: "\n\n## 聊天命令\n", false: "\n\n## Chat commands\n"}[zh])
	for _, command := range commands {
		fmt.Fprintf(b, "\n### `%s`\n\n%s\n\n- %s: `%s`\n- %s: `%s`\n", command.Invocation,
			localized(zh, command.Description, command.DescriptionZH),
			map[bool]string{true: "用法", false: "Usage"}[zh], command.Usage,
			map[bool]string{true: "来源", false: "Source"}[zh], command.Source)
		if len(command.PrivilegedWhen) > 0 {
			fmt.Fprintf(b, "- %s: `%s`\n", map[bool]string{true: "需要管理员权限的子操作", false: "Admin-only sub-operations"}[zh], strings.Join(command.PrivilegedWhen, "`, `"))
		}
		renderActionContract(b, command.Permission, command.ReadOnly, command.SideEffects, command.Fallback, command.Availability, command.Parameters, zh)
	}
}

func renderSkillCapabilities(b *strings.Builder, skills []AgentSkillCapability, zh bool) {
	if len(skills) == 0 {
		return
	}
	b.WriteString(map[bool]string{true: "\n\n## Skills\n", false: "\n\n## Skills\n"}[zh])
	for _, skill := range skills {
		name := skill.DisplayName
		if name == "" {
			name = skill.Name
		}
		fmt.Fprintf(b, "\n### `%s` — %s\n\n%s\n", skill.Invocation, name, skill.Description)
		renderActionContract(b, skill.Permission, skill.ReadOnly, skill.SideEffects, defaultRejectFallback(), skill.Availability, skill.Parameters, zh)
	}
}

func renderActionContract(b *strings.Builder, permission CapabilityPermissionLevel, readOnly bool, effects []CapabilitySideEffect, fallback CapabilityFallback, availability CapabilityAvailability, parameters []CapabilityParameter, zh bool) {
	fmt.Fprintf(b, "- %s: `%s` — %s\n", map[bool]string{true: "可用性", false: "Availability"}[zh], availability.State, localized(zh, availability.Reason, availability.ReasonZH))
	fmt.Fprintf(b, "- %s: `%s`\n", map[bool]string{true: "权限", false: "Permission"}[zh], permission)
	fmt.Fprintf(b, "- %s: `%t`\n", map[bool]string{true: "只读", false: "Read-only"}[zh], readOnly)
	if len(effects) > 0 {
		var values []string
		for _, effect := range effects {
			values = append(values, fmt.Sprintf("`%s` (%s)", effect.Kind, localized(zh, effect.Description, effect.DescriptionZH)))
		}
		fmt.Fprintf(b, "- %s: %s\n", map[bool]string{true: "写入/外部副作用", false: "Write/external side effects"}[zh], strings.Join(values, "; "))
	}
	fmt.Fprintf(b, "- %s: `%s` — %s\n", map[bool]string{true: "退化行为", false: "Fallback"}[zh], fallback.Mode, localized(zh, fallback.Description, fallback.DescriptionZH))
	if len(parameters) > 0 {
		b.WriteString(map[bool]string{true: "- 参数：\n", false: "- Parameters:\n"}[zh])
		for _, parameter := range parameters {
			required := map[bool]string{true: "required", false: "optional"}[parameter.Required]
			if zh {
				required = map[bool]string{true: "必填", false: "可选"}[parameter.Required]
			}
			fmt.Fprintf(b, "  - `%s` (`%s`, %s): %s", parameter.Name, parameter.Type, required, localized(zh, parameter.Description, parameter.DescriptionZH))
			if len(parameter.AllowedValues) > 0 {
				fmt.Fprintf(b, " %s: `%s`", map[bool]string{true: "允许值", false: "Values"}[zh], strings.Join(parameter.AllowedValues, "`, `"))
			}
			b.WriteByte('\n')
		}
	}
}

func renderRuntimeCapabilities(b *strings.Builder, adapters []RuntimeAdapterCapabilities, zh bool) {
	if len(adapters) == 0 {
		return
	}
	b.WriteString(map[bool]string{true: "\n\n## 运行态适配器能力\n", false: "\n\n## Runtime adapter capabilities\n"}[zh])
	for _, adapter := range adapters {
		fmt.Fprintf(b, "\n### `%s:%s` — `%s`\n\n", adapter.Kind, adapter.Name, adapter.State)
		if adapter.Reason != "" {
			fmt.Fprintf(b, "%s\n\n", adapter.Reason)
		}
		for _, capability := range adapter.Capabilities {
			fmt.Fprintf(b, "- `%s`: `%s` — %s; %s `%s`: %s\n", capability.ID, capability.Availability.State,
				localized(zh, capability.Description, capability.DescriptionZH),
				map[bool]string{true: "退化", false: "fallback"}[zh], capability.Fallback.Mode,
				localized(zh, capability.Fallback.Description, capability.Fallback.DescriptionZH))
		}
	}
}

func renderManifestConfiguration(b *strings.Builder, manifest AgentCapabilityManifest, zh bool) {
	catalog := manifest.Configuration
	if len(catalog.Capabilities) == 0 && len(catalog.Options) == 0 {
		return
	}
	b.WriteString(map[bool]string{true: "\n\n## 配置能力\n", false: "\n\n## Configuration capabilities\n"}[zh])
	if manifest.Query == "" {
		fmt.Fprintf(b, "\n%s %d · %s %d\n", map[bool]string{true: "能力组", false: "Capability groups"}[zh], len(catalog.Capabilities), map[bool]string{true: "配置项", false: "Options"}[zh], len(catalog.Options))
		for _, capability := range catalog.Capabilities {
			fmt.Fprintf(b, "\n- `%s` — %s", capability.ID, localized(zh, capability.Description, capability.DescriptionZH))
		}
		b.WriteString(map[bool]string{true: "\n\n使用 `--search` 获取精确配置位置、要求、默认来源、允许值和示例。", false: "\n\nUse `--search` for exact placement, requirements, default source, allowed values, and examples."}[zh])
		return
	}
	rendered := RenderConfigCatalogMarkdown(catalog, map[bool]string{true: "zh", false: "en"}[zh])
	for _, line := range strings.Split(rendered, "\n") {
		if strings.HasPrefix(line, "#") {
			line = "#" + line
		}
		b.WriteByte('\n')
		b.WriteString(line)
	}
}

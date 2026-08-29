package core

import (
	"os"
	"sort"
	"strings"
)

const (
	bridgeCapabilitiesSnapshotType  = "capabilities_snapshot"
	bridgeCapabilitiesSnapshotProto = "capabilities_snapshot_v1"
	bridgeCommandArgsModeText       = "text"
	bridgeCommandSourceBuiltin      = "builtin"
	bridgeCommandSourceCustom       = "custom"
)

// CurrentCommit is set by main at startup so bridge clients can inspect the
// host binary that produced a capability snapshot.
var CurrentCommit string

// CurrentBuildTime is set by main at startup so bridge clients can compare
// host snapshots without reverse-engineering git-describe version strings.
var CurrentBuildTime string

type bridgeCapabilitiesSnapshot struct {
	Type     string                      `json:"type"`
	Version  int                         `json:"v"`
	Host     bridgeCapabilitiesHost      `json:"host"`
	Projects []bridgeProjectCapabilities `json:"projects"`
}

type bridgeCapabilitiesHost struct {
	ID               string `json:"id"`
	Hostname         string `json:"hostname,omitempty"`
	CCConnectVersion string `json:"cc_connect_version,omitempty"`
	Commit           string `json:"commit,omitempty"`
	BuildTime        string `json:"build_time,omitempty"`
}

type bridgeProjectCapabilities struct {
	Project  string                   `json:"project"`
	Commands []bridgePublishedCommand `json:"commands"`
}

type bridgePublishedCommand struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	Source            string `json:"source"`
	RequiresWorkspace bool   `json:"requires_workspace"`
	ArgsMode          string `json:"args_mode"`
}

// GetBridgePublishedCommands projects the unified Agent Capability Manifest's
// chat-command contracts onto the legacy Bridge command shape. Skills remain a
// separate Manifest section because capabilities_snapshot_v1 has no Skill
// model; commands, permissions, disabled policy, and availability therefore
// still come from one runtime source of truth.
func (e *Engine) GetBridgePublishedCommands() []bridgePublishedCommand {
	var commands []bridgePublishedCommand
	for _, capability := range e.agentCommandCapabilities("") {
		if capability.Availability.State == CapabilityUnavailable {
			continue
		}
		source := bridgeCommandSourceBuiltin
		if strings.HasPrefix(capability.Source, "custom-") {
			source = bridgeCommandSourceCustom
		}
		description := capability.Description
		if e.i18n.IsZhLike() && capability.DescriptionZH != "" {
			description = capability.DescriptionZH
		}
		commands = append(commands, bridgePublishedCommand{
			Name:              capability.ID,
			Description:       description,
			Source:            source,
			RequiresWorkspace: false,
			ArgsMode:          bridgeCommandArgsModeText,
		})
	}
	sort.Slice(commands, func(i, j int) bool { return strings.ToLower(commands[i].Name) < strings.ToLower(commands[j].Name) })
	return commands
}

func (bs *BridgeServer) buildCapabilitiesSnapshot() bridgeCapabilitiesSnapshot {
	hostName, _ := os.Hostname()
	projects := make([]bridgeProjectCapabilities, 0, len(bs.engines))

	bs.enginesMu.RLock()
	projectNames := make([]string, 0, len(bs.engines))
	for projectName := range bs.engines {
		projectNames = append(projectNames, projectName)
	}
	sort.Strings(projectNames)
	for _, projectName := range projectNames {
		ref := bs.engines[projectName]
		if ref == nil || ref.engine == nil {
			continue
		}
		projects = append(projects, bridgeProjectCapabilities{
			Project:  projectName,
			Commands: ref.engine.GetBridgePublishedCommands(),
		})
	}
	bs.enginesMu.RUnlock()

	return bridgeCapabilitiesSnapshot{
		Type:    bridgeCapabilitiesSnapshotType,
		Version: 1,
		Host: bridgeCapabilitiesHost{
			ID:               hostName,
			Hostname:         hostName,
			CCConnectVersion: CurrentVersion,
			Commit:           CurrentCommit,
			BuildTime:        CurrentBuildTime,
		},
		Projects: projects,
	}
}

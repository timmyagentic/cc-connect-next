package main

import (
	"context"
	"fmt"
	"strings"

	ccconfig "github.com/timmyagentic/cc-connect-next/config"
	"github.com/timmyagentic/cc-connect-next/core"
)

type migrationNotifyHints struct {
	Project  string
	Platform string
	UserID   string
}

type migrationNotifyTarget struct {
	ProjectName string
	Platform    ccconfig.PlatformConfig
	UserID      string
	Message     string
}

func resolveMigrationNotifyTarget(configPath string, hints migrationNotifyHints) (*migrationNotifyTarget, string, error) {
	cfg, err := ccconfig.Load(configPath)
	if err != nil {
		return nil, "", fmt.Errorf("load migration notification config: %w", err)
	}

	hints.Project = strings.TrimSpace(hints.Project)
	hints.Platform = strings.TrimSpace(hints.Platform)
	hints.UserID = strings.TrimSpace(hints.UserID)
	var candidates []migrationNotifyTarget
	for _, project := range cfg.Projects {
		if hints.Project != "" && project.Name != hints.Project {
			continue
		}
		for _, platform := range project.Platforms {
			if platform.Type != "feishu" && platform.Type != "lark" {
				continue
			}
			if hints.Platform != "" && platform.Type != hints.Platform {
				continue
			}
			userID := hints.UserID
			if userID == "" && len(project.Platforms) == 1 {
				userID = singleConfiguredID(project.AdminFrom)
			}
			if userID == "" {
				allowFrom, _ := platform.Options["allow_from"].(string)
				userID = singleConfiguredID(allowFrom)
			}
			if userID != "" {
				candidates = append(candidates, migrationNotifyTarget{
					ProjectName: project.Name,
					Platform:    platform,
					UserID:      userID,
					Message:     core.NewI18n(configLanguage(cfg.Language)).T(core.MsgMigrationComplete),
				})
			}
		}
	}

	if len(candidates) == 1 {
		return &candidates[0], "", nil
	}
	if len(candidates) > 1 {
		return nil, "notification target is ambiguous; use --notify-project, --notify-user, and if needed --notify-platform", nil
	}
	return nil, "no unique Feishu/Lark operator could be verified; use --notify-project and --notify-user", nil
}

func singleConfiguredID(raw string) string {
	seen := make(map[string]struct{})
	for _, part := range strings.Split(raw, ",") {
		id := strings.TrimSpace(part)
		if id == "" || id == "*" {
			continue
		}
		seen[id] = struct{}{}
	}
	if len(seen) != 1 {
		return ""
	}
	for id := range seen {
		return id
	}
	return ""
}

func sendMigrationComplete(ctx context.Context, target *migrationNotifyTarget, dataDir string) error {
	if target == nil {
		return nil
	}
	opts := make(map[string]any, len(target.Platform.Options)+2)
	for key, value := range target.Platform.Options {
		opts[key] = value
	}
	opts["cc_data_dir"] = dataDir
	opts["cc_project"] = target.ProjectName
	platform, err := core.CreatePlatform(target.Platform.Type, opts)
	if err != nil {
		return err
	}
	sender, ok := platform.(core.DirectUserSender)
	if !ok {
		return fmt.Errorf("platform %q does not support private user messages", target.Platform.Type)
	}
	return sender.SendDirectUser(ctx, target.UserID, target.Message)
}

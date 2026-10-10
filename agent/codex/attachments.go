package codex

import (
	"fmt"
	"strings"

	"github.com/timmyagentic/cc-connect-next/core"
)

func stageCodexImages(workDir, prompt, source string, images []core.ImageAttachment) (string, []string, error) {
	paths, err := core.StageImagesToDisk(workDir, images)
	if err != nil {
		return "", nil, fmt.Errorf("%s: stage images: %w", source, err)
	}
	if len(paths) > 0 && strings.TrimSpace(prompt) == "" {
		prompt = "Please analyze the attached image(s)."
	}
	return prompt, paths, nil
}

// Do not launch a text-only turn after silently dropping a file. Successfully
// written files remain available for retry; failed batches never reach Codex.
func stageCodexFiles(workDir, prompt string, files []core.FileAttachment) (string, error) {
	paths, err := core.StageFilesToDisk(workDir, files)
	if err != nil {
		return "", err
	}
	return core.AppendFileRefs(prompt, paths), nil
}

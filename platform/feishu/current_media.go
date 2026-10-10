package feishu

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/timmyagentic/cc-connect-next/core"
)

// Attachment status is agent input; use the canonical English copy because a
// platform does not own the engine's per-session language setting.
func mediaUnavailable(key core.MsgKey) string {
	return core.NewI18n(core.LangEnglish).T(key)
}

func (p *Platform) parseCurrentPost(ctx context.Context, messageID, raw string) ([]string, []core.ImageAttachment, []core.FileAttachment) {
	post := decodePost(raw)
	if post == nil {
		return nil, nil, nil
	}
	text, images, files := p.extractPostPartsContext(ctx, messageID, post, true)
	for _, file := range post.Files {
		if file.IsFolder {
			text = append(text, mediaUnavailable(core.MsgFolderUnsupported))
			continue
		}
		// Malformed file metadata must remain visible even without a resource key.
		if file.FileKey == "" {
			text = append(text, mediaUnavailable(core.MsgFileUnavailable))
			continue
		}
		data, err := p.downloadResourceContext(ctx, messageID, file.FileKey, "file")
		if err != nil {
			slog.Warn(p.tag()+": download post file failed", "error", core.RedactToken(err.Error(), p.appSecret))
			text = append(text, mediaUnavailable(core.MsgFileUnavailable))
			continue
		}
		files = append(files, core.FileAttachment{FileName: file.FileName, MimeType: http.DetectContentType(data), Data: data, MessageID: messageID})
	}
	return text, images, files
}

// Only called for the admitted current message, never for history or quotes.
func (p *Platform) downloadCurrentVideo(ctx context.Context, messageID, fileKey, imageKey, fileName string) (string, []core.ImageAttachment, []core.FileAttachment) {
	text := "[video]"
	var images []core.ImageAttachment
	var files []core.FileAttachment
	data, err := p.downloadResourceContext(ctx, messageID, fileKey, "file")
	if err != nil {
		slog.Warn(p.tag()+": download video failed", "error", core.RedactToken(err.Error(), p.appSecret))
		text = mediaUnavailable(core.MsgVideoUnavailable)
	} else {
		mimeType := http.DetectContentType(data)
		if strings.TrimSpace(fileName) == "" {
			fileName = "video"
			switch mimeType {
			case "video/mp4":
				fileName += ".mp4"
			case "video/webm":
				fileName += ".webm"
			case "video/quicktime":
				fileName += ".mov"
			default:
				fileName += ".bin"
			}
		}
		files = append(files, core.FileAttachment{FileName: fileName, MimeType: mimeType, Data: data, MessageID: messageID})
	}
	if imageKey != "" {
		cover, mimeType, err := p.downloadImageContext(ctx, messageID, imageKey)
		if err != nil {
			slog.Warn(p.tag()+": download video cover failed", "error", core.RedactToken(err.Error(), p.appSecret))
			text += "\n" + mediaUnavailable(core.MsgImageUnavailable)
		} else {
			images = append(images, core.ImageAttachment{MimeType: mimeType, Data: cover})
		}
	}
	return text, images, files
}

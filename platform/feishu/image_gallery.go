package feishu

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	"github.com/timmyagentic/cc-connect-next/core"
)

// Five images keep the two-column gallery readable on mobile. This is our
// delivery policy, below the native bisect component's six-image capacity.
const imageGalleryBatchSize = 5

func (p *Platform) ImageBatchSize() int {
	if !p.useInteractiveCard {
		return 1
	}
	return imageGalleryBatchSize
}

func (p *Platform) SendImageBatch(ctx context.Context, rctx any, images []core.ImageAttachment) error {
	rc, ok := rctx.(replyContext)
	if !ok {
		return fmt.Errorf("%s: image gallery: invalid reply context type %T", p.tag(), rctx)
	}
	if !p.useInteractiveCard || len(images) == 0 || len(images) > imageGalleryBatchSize {
		return fmt.Errorf("%s: image gallery requires cards enabled and 1-%d images", p.tag(), imageGalleryBatchSize)
	}
	keys := make([]galleryImage, 0, len(images))
	for i, img := range images {
		if err := ctx.Err(); err != nil {
			return err
		}
		key, err := p.uploadImageKey(ctx, img.Data)
		if err != nil {
			return fmt.Errorf("%s: image gallery upload %d/%d: %w", p.tag(), i+1, len(images), err)
		}
		keys = append(keys, galleryImage{Key: key})
	}
	content, err := json.Marshal(struct {
		Schema string `json:"schema"`
		Body   struct {
			Elements []galleryElement `json:"elements"`
		} `json:"body"`
	}{Schema: "2.0", Body: struct {
		Elements []galleryElement `json:"elements"`
	}{Elements: []galleryElement{{Tag: "img_combination", Mode: "bisect", Images: keys}}}})
	if err != nil {
		return fmt.Errorf("%s: encode image gallery: %w", p.tag(), err)
	}
	// Allocate once per group, outside transport retries. A lost response must
	// not create another visible card. A new explicit send remains a new send.
	id := rand.Text()
	if p.shouldUseThreadOrReplyAPI(rc) {
		body := p.buildReplyMessageReqBody(rc, larkim.MsgTypeInteractive, string(content))
		body.Uuid = &id
		_, err = p.replyMessageWithBody(ctx, rc.messageID, body)
	} else {
		_, err = p.createMessageToReceiveIDWithUUID(ctx, rc.chatID, larkim.CreateMessageV1ReceiveIDTypeChatId, larkim.MsgTypeInteractive, string(content), "send image gallery", id)
	}
	return err
}

type galleryImage struct {
	Key string `json:"img_key"`
}

type galleryElement struct {
	Tag    string         `json:"tag"`
	Mode   string         `json:"combination_mode"`
	Images []galleryImage `json:"img_list"`
}

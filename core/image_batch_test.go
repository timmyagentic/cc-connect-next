package core

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type galleryTestPlatform struct {
	stubMediaPlatform
	groups   [][]ImageAttachment
	contexts []any
	size     int
	failAt   int
}

func (p *galleryTestPlatform) ImageBatchSize() int { return p.size }
func (p *galleryTestPlatform) SendImageBatch(_ context.Context, ctx any, images []ImageAttachment) error {
	if p.failAt > 0 && len(p.groups)+1 == p.failAt {
		return errors.New("gallery rejected")
	}
	p.groups = append(p.groups, append([]ImageAttachment(nil), images...))
	p.contexts = append(p.contexts, ctx)
	return p.stubPlatformEngine.Send(context.Background(), ctx, fmt.Sprintf("gallery:%d", len(images)))
}
func galleryTestImages(n int) []ImageAttachment {
	images := make([]ImageAttachment, n)
	for i := range images {
		images[i] = ImageAttachment{FileName: fmt.Sprintf("%d.png", i), Data: []byte{byte(i)}}
	}
	return images
}
func TestEngineSendImages_GalleryInsteadOfIndividualMessages(t *testing.T) {
	for _, n := range []int{1, 2, 5, 6, 10, 11} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			p := &galleryTestPlatform{size: 5}
			p.n = "test"
			e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
			e.interactiveStates["s"] = &interactiveState{platform: p, replyCtx: "topic-context"}
			images := galleryTestImages(n)
			if err := e.SendToSessionWithAttachments("s", "", images, nil, nil, false); err != nil {
				t.Fatal(err)
			}
			if n == 1 {
				if len(p.images) != 1 || len(p.groups) != 0 {
					t.Fatal("single image changed")
				}
				return
			}
			if len(p.images) != 0 || len(p.groups) != (n+4)/5 {
				t.Fatalf("single=%d groups=%d", len(p.images), len(p.groups))
			}
			var got []ImageAttachment
			for i, g := range p.groups {
				if len(g) > 5 || p.contexts[i] != "topic-context" {
					t.Fatal("batch size or target changed")
				}
				got = append(got, g...)
			}
			if !reflect.DeepEqual(got, images) {
				t.Fatal("image order or bytes changed")
			}
		})
	}
}
func TestEngineSendImages_PartialFailureDoesNotReplayConfirmedGroups(t *testing.T) {
	p := &galleryTestPlatform{size: 5, failAt: 2}
	p.n = "test"
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.interactiveStates["s"] = &interactiveState{platform: p}
	err := e.SendToSessionWithAttachments("s", "", galleryTestImages(11), []FileAttachment{{FileName: "after.txt"}}, nil, false)
	var partial *ImageDeliveryError
	if !errors.As(err, &partial) || partial.Sent != 5 || partial.Total != 11 {
		t.Fatalf("missing partial progress: %v", err)
	}
	if len(p.groups) != 1 || len(p.images) != 0 || len(p.files) != 0 {
		t.Fatal("failed delivery replayed or continued")
	}
}
func TestEngineSendImages_FallbackAndDisabled(t *testing.T) {
	p := &galleryTestPlatform{size: 1}
	p.n = "test"
	e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
	e.interactiveStates["s"] = &interactiveState{platform: p}
	if err := e.SendToSessionWithAttachments("s", "", galleryTestImages(3), nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if len(p.images) != 3 || len(p.groups) != 0 {
		t.Fatal("disabled grouping must retain individual sends")
	}
	e.attachmentSendEnabled = false
	if err := e.SendToSessionWithAttachments("s", "", galleryTestImages(3), nil, nil, false); !errors.Is(err, ErrAttachmentSendDisabled) {
		t.Fatal(err)
	}
	if len(p.images) != 3 {
		t.Fatal("disabled attachments leaked")
	}
}
func TestEngineSendImages_WorkspaceDeliveryUsesGallery(t *testing.T) {
	e := newTestEngineWithMultiWorkspaceAgent(t, t.TempDir())
	p := &galleryTestPlatform{size: 5}
	p.n = "test"
	e.platforms = []Platform{p}
	e.interactiveStates["s"] = &interactiveState{platform: p, replyCtx: "topic"}
	if err := e.SendToSessionInWorkDir("s", "", galleryTestImages(10), nil, t.TempDir(), nil, false); err != nil {
		t.Fatal(err)
	}
	if len(p.groups) != 2 || len(p.images) != 0 {
		t.Fatalf("groups=%d singles=%d", len(p.groups), len(p.images))
	}
}

func TestImageGallery_ManifestSearchAndFallback(t *testing.T) {
	for _, size := range []int{1, 5} {
		p := &galleryTestPlatform{size: size}
		p.n = "test"
		e := NewEngine("test", &stubAgent{}, []Platform{p}, "", LangEnglish)
		e.platformReady[p] = true
		for _, query := range []string{"image_batches", "图集"} {
			m := e.QueryAgentCapabilityManifest("", query, false)
			adapter := findRuntimeAdapter(t, m.Runtime, "platform", "test")
			feature := findRuntimeFeature(t, adapter.Capabilities, "image_batches")
			want := CapabilityAvailable
			if size == 1 {
				want = CapabilityUnavailable
			}
			if feature.Availability.State != want || feature.Fallback.Mode != "individual-images" {
				t.Fatalf("misleading manifest: %+v", feature)
			}
		}
	}
}

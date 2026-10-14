package feishu

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type slowUnreadReminderTransport struct{ inner http.RoundTripper }

func (r slowUnreadReminderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "/messages/") {
		timer := time.NewTimer(800 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
	return r.inner.RoundTrip(req)
}

func TestUnreadReminderSlowBatchStillSendsValidatedCandidates(t *testing.T) {
	f := newReminderFixture(t)
	for i := 0; i < maxReminderBatch; i++ {
		if err := f.notice(fmt.Sprintf("om_trigger_%d", i), "omt_topic"); err != nil {
			t.Fatal(err)
		}
		f.now = f.now.Add(time.Second)
	}
	f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(f.p.domain),
		lark.WithHttpClient(&http.Client{Transport: slowUnreadReminderTransport{inner: http.DefaultTransport}}))
	f.now = f.now.Add(time.Hour)
	// Exercise the real query deadline: each individual request succeeds in
	// 800ms, but twenty records require more than a single query phase.
	f.p.processUnreadReminders(context.Background())
	if len(f.reads) < 2 {
		t.Fatalf("setup: deadline must allow several successful unread checks, got %d", len(f.reads))
	}
	t.Logf("successful unread queries=%d private sends=%d", len(f.reads), f.private)
	for _, record := range f.p.reminders.records {
		if record.Attempts != 0 {
			t.Errorf("batch deadline spent API retry allowance: %+v", record)
		}
	}
	if f.private == 0 {
		t.Fatal("all successfully validated unread candidates were discarded when a later item exhausted the shared batch deadline")
	}
	// Finish the remaining records without the synthetic latency. Previously
	// sent records must not be re-queried or sent again on the next tick/reload.
	f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(f.p.domain))
	f.tick()
	if f.private != 2 || len(f.reads) != maxReminderBatch {
		t.Fatalf("remaining batch did not make progress: private=%d read queries=%d", f.private, len(f.reads))
	}
	f.reload()
	f.tick()
	if f.private != 2 {
		t.Fatal("resent a successfully delivered batch after reload")
	}
}

type blockedUnreadReminderTransport struct {
	blocked chan struct{}
	once    sync.Once
	target  string
}

func (r *blockedUnreadReminderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	target := r.target
	if target == "" {
		target = "om_second"
	}
	if req.Method == http.MethodGet && strings.HasSuffix(req.URL.Path, "/messages/"+target) {
		r.once.Do(func() { close(r.blocked) })
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return http.DefaultTransport.RoundTrip(req)
}

func TestUnreadReminderFirstQueryTimeoutDoesNotStarveLaterRecords(t *testing.T) {
	f := newReminderFixture(t)
	for _, trigger := range []string{"om_first", "om_second"} {
		if err := f.notice(trigger, "omt_topic"); err != nil {
			t.Fatal(err)
		}
		f.now = f.now.Add(time.Second)
	}
	f.now = f.now.Add(time.Hour)
	transport := &blockedUnreadReminderTransport{blocked: make(chan struct{}), target: "om_first"}
	f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(f.p.domain), lark.WithHttpClient(&http.Client{Transport: transport}))
	f.tick()
	for _, record := range f.p.reminders.records {
		wantAttempts := 0
		if record.TriggerID == "om_first" {
			wantAttempts = 1
			if !record.Next.After(f.now.Add(time.Minute)) {
				t.Fatal("full-budget timeout must yield to later records on the next tick")
			}
		}
		if record.Attempts != wantAttempts || record.State != "pending" {
			t.Fatalf("unexpected state after first-query timeout: %+v", record)
		}
	}
	f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(f.p.domain))
	f.now = f.now.Add(time.Minute)
	f.tick()
	if f.private != 1 {
		t.Fatalf("later record starved: private=%d", f.private)
	}
	f.now = f.now.Add(time.Minute)
	f.tick()
	if f.private != 2 {
		t.Fatalf("timed-out record did not recover after backoff: private=%d", f.private)
	}
}

func TestUnreadReminderCancellationStillSuppressesValidatedBatch(t *testing.T) {
	for _, mode := range []string{"parent cancel", "worker stop"} {
		t.Run(mode, func(t *testing.T) {
			f := newReminderFixture(t)
			for _, trigger := range []string{"om_first", "om_second"} {
				if err := f.notice(trigger, "omt_topic"); err != nil {
					t.Fatal(err)
				}
				f.now = f.now.Add(time.Second)
			}
			f.now = f.now.Add(time.Hour)
			transport := &blockedUnreadReminderTransport{blocked: make(chan struct{})}
			f.p.client = lark.NewClient(f.p.appID, "test", lark.WithOpenBaseUrl(f.p.domain), lark.WithHttpClient(&http.Client{Transport: transport}))
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			if mode == "worker stop" {
				if err := f.p.startUnreadReminders(); err != nil {
					t.Fatal(err)
				}
				defer f.p.stopUnreadReminders()
			} else {
				go func() { defer close(done); f.p.processUnreadReminders(ctx) }()
			}
			select {
			case <-transport.blocked:
			case <-time.After(5 * time.Second):
				t.Fatal("second query did not start")
			}
			if mode == "worker stop" {
				f.p.stopUnreadReminders()
			} else {
				cancel()
				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("parent cancellation did not stop the batch")
				}
			}
			if len(f.reads) != 1 || f.private != 0 {
				t.Fatalf("cancellation bypassed: read=%d private=%d", len(f.reads), f.private)
			}
			for _, record := range f.p.reminders.records {
				if record.State != "pending" || record.Attempts != 0 {
					t.Fatalf("cancellation changed pending record: %+v", record)
				}
			}
		})
	}
}

package core

import (
	"testing"
)

type cardHookTestPlatform struct {
	stubPlatformEngine
	handler func(string, HookEvent) error
}

func (p *cardHookTestPlatform) SetCardActionHandler(handler func(string, HookEvent) error) {
	p.handler = handler
}

func TestEngineWiresNamedCardHooksAndClearsRemovedHandler(t *testing.T) {
	p := &cardHookTestPlatform{stubPlatformEngine: stubPlatformEngine{n: "fixture"}}
	e := &Engine{platforms: []Platform{p}}
	e.SetHooks(NewHookManager("example", nil, "", "", ""))
	if p.handler == nil {
		t.Fatal("platform card hook dispatcher was not wired")
	}
	if err := p.handler("unregistered", HookEvent{Event: HookEventCardAction}); err == nil {
		t.Fatal("unregistered card hook was accepted")
	}
	e.SetHooks(nil)
	if p.handler != nil {
		t.Fatal("removed hooks still expose a dispatcher")
	}
}

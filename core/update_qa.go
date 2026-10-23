//go:build updateqa

package core

import (
	"github.com/timmyagentic/cc-connect-next/internal/appfeatures"
	"github.com/timmyagentic/cc-connect-next/internal/updatechannel"
)

// NewFixtureUpdateService lets the opt-in real-client harness inject only its
// release transport. The production plan ownership and progress bridge remain
// unchanged. This constructor is absent from normal product builds.
func NewFixtureUpdateService(service *appfeatures.UpdateService, channel updatechannel.Channel) UpdateService {
	return &foundationUpdateService{service: service, channel: channel}
}

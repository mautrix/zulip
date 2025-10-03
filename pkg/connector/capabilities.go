package connector

import (
	"context"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-zulip/pkg/zid"
)

func (zc *ZulipConnector) GetBridgeInfoVersion() (info, capabilities int) {
	return 1, 1
}

func (zc *ZulipConnector) GetCapabilities() *bridgev2.NetworkGeneralCapabilities {
	return &bridgev2.NetworkGeneralCapabilities{
		DisappearingMessages:    false,
		AggressiveUpdateInfo:    false,
		ImplicitReadReceipts:    false,
		OutgoingMessageTimeouts: nil,
		Provisioning:            bridgev2.ProvisioningCapabilities{},
	}
}

func (zc *ZulipClient) GetCapabilities(ctx context.Context, portal *bridgev2.Portal) *event.RoomFeatures {
	caps := &event.RoomFeatures{
		ID:     "fi.mau.zulip.capabilities.2025_10_02",
		Thread: event.CapLevelFullySupported,
	}
	_, userIDs, _ := zid.ParsePortalID(portal.ID)
	if userIDs != nil {
		caps.ID += "+dm"
		caps.Thread = event.CapLevelUnsupported
	}
	return caps
}

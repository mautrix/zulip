package connector

import (
	"go.mau.fi/util/exslices"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func (zc *ZulipClient) makeEventSender(id int) bridgev2.EventSender {
	return bridgev2.EventSender{
		IsFromMe:    id == zc.ownUserID,
		SenderLogin: zid.MakeUserLoginID(id),
		Sender:      zid.MakeUserID(id),
	}
}

func (zc *ZulipClient) makeChannelPortalKey(streamID int) (pk networkid.PortalKey) {
	if zc.UserLogin.Bridge.Config.SplitPortals {
		pk.Receiver = zc.UserLogin.ID
	}
	pk.ID = zid.MakeChannelPortalID(streamID)
	return pk
}

func (zc *ZulipClient) makePortalKey(message events.MessageData) (pk networkid.PortalKey) {
	if zc.UserLogin.Bridge.Config.SplitPortals || message.StreamID == 0 {
		pk.Receiver = zc.UserLogin.ID
	}
	if message.StreamID != 0 {
		pk.ID = zid.MakeChannelPortalID(message.StreamID)
	} else {
		pk.ID = zid.MakeDMPortalID(exslices.CastFuncFilter(message.DisplayRecipient.Users, func(from events.DisplayRecipientObject) (int, bool) {
			if from.ID == zc.ownUserID {
				return 0, false
			}
			return from.ID, true
		}))
	}
	return pk
}

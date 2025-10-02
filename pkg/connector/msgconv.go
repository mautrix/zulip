package connector

import (
	"context"

	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func (zc *ZulipClient) convertZulipMessage(
	ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data *events.MessageData,
) (*bridgev2.ConvertedMessage, error) {
	var threadRootID *networkid.MessageID
	if data.Subject != "" {
		threadRootID = ptr.Ptr(makeTopicMessageID(data.Subject))
	}
	return &bridgev2.ConvertedMessage{
		ThreadRoot: threadRootID,
		Parts: []*bridgev2.ConvertedMessagePart{{
			Type: event.EventMessage,
			Content: &event.MessageEventContent{
				MsgType: event.MsgText,
				Body:    data.Content,
			},
		}},
	}, nil
}

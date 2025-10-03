package connector

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip/messages"
	"go.mau.fi/mautrix-zulip/pkg/zulip/messages/recipient"
)

func (zc *ZulipClient) HandleMatrixMessage(ctx context.Context, msg *bridgev2.MatrixMessage) (message *bridgev2.MatrixMessageResponse, err error) {
	channelID, userIDs, err := zid.ParsePortalID(msg.Portal.ID)
	if err != nil {
		return nil, err
	}
	srv := messages.NewService(zc.Client)
	var resp *messages.SendMessageResponse
	var threadRootID networkid.MessageID
	var topicID string
	if msg.ThreadRoot != nil {
		threadRootID = msg.ThreadRoot.ID
		if msg.ThreadRoot.ThreadRoot != "" {
			threadRootID = msg.ThreadRoot.ThreadRoot
		}
		topicID, _ = zid.ParseMessageID(threadRootID)
		if topicID == "" {
			return nil, fmt.Errorf("invalid thread root")
		}
	}
	if channelID != 0 {
		resp, err = srv.SendMessageToChannelTopic(ctx, recipient.ToChannel(channelID), topicID, msg.Content.Body)
	} else {
		resp, err = srv.SendMessageToUsers(ctx, recipient.ToUsers(userIDs), msg.Content.Body)
	}
	if err != nil {
		return nil, err
	}
	return &bridgev2.MatrixMessageResponse{
		DB: &database.Message{
			ID:         zid.MakeMessageID(resp.ID),
			SenderID:   zid.MakeUserID(zc.ownUserID),
			ThreadRoot: threadRootID,
		},
		StreamOrder: int64(resp.ID),
	}, nil
}

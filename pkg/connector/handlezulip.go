package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-zulip/pkg/msgconv"
	"go.mau.fi/mautrix-zulip/pkg/msgconv/zulipemoji"
	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func (zc *ZulipClient) handleZulipEvent(ctx context.Context, rawEvt events.Event) bool {
	log := zerolog.Ctx(ctx)
	log.Trace().Any("data", rawEvt).Msg("Event data")
	switch evt := rawEvt.(type) {
	case *events.UserTopic:
		if evt.TopicName == "" {
			return true
		}
		return zc.UserLogin.QueueRemoteEvent(zc.makeTopicUpsert(evt.TopicName, evt.StreamID)).Success
	case *events.Message:
		if evt.Message.StreamID != 0 && evt.Message.Subject != "" {
			if !zc.UserLogin.QueueRemoteEvent(zc.makeTopicUpsert(evt.Message.Subject, evt.Message.StreamID)).Success {
				return false
			}
		}
		return zc.UserLogin.QueueRemoteEvent(&simplevent.Message[*events.MessageData]{
			EventMeta: simplevent.EventMeta{
				Type: bridgev2.RemoteEventMessage,
				LogContext: func(c zerolog.Context) zerolog.Context {
					return c.
						Int("evt_id", evt.ID).
						Int("msg_id", evt.Message.ID).
						Int("stream_id", evt.Message.StreamID).
						Int("recipient_id", evt.Message.RecipientID)
				},
				PortalKey:    zc.makePortalKey(evt.Message),
				Sender:       zc.makeEventSender(evt.Message.SenderID),
				CreatePortal: true,
				Timestamp:    time.Unix(int64(evt.Message.Timestamp), 0),
				StreamOrder:  int64(evt.Message.ID),
			},
			Data:          &evt.Message,
			ID:            zid.MakeMessageID(evt.Message.ID),
			TransactionID: networkid.TransactionID(evt.LocalID),
			ConvertMessageFunc: func(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, data *events.MessageData) (*bridgev2.ConvertedMessage, error) {
				return msgconv.ToMatrix(ctx, portal, intent, zc.UserLogin, data)
			},
		}).Success
	//case *events.DeleteMessage:
	//	var portalKey networkid.PortalKey
	//	if evt.StreamID != nil {
	//		portalKey = zc.makeChannelPortalKey(*evt.StreamID)
	//	} else {
	//		// TODO
	//	}
	//	return zc.UserLogin.QueueRemoteEvent(&simplevent.MessageRemove{
	//		EventMeta: simplevent.EventMeta{
	//			Type:       bridgev2.RemoteEventMessageRemove,
	//			LogContext: nil,
	//			PortalKey:  portalKey,
	//		},
	//		TargetMessage: "",
	//		OnlyForMe:     false,
	//	})
	case *events.Reaction:
		part, err := zc.Main.Bridge.DB.Message.GetPartByID(ctx, zc.UserLogin.ID, zid.MakeMessageID(evt.MessageID), "")
		if err != nil {
			log.Err(err).Msg("Failed to get portal for reaction")
			return false
		} else if part == nil {
			log.Warn().Int("target_message_id", evt.MessageID).Msg("Reaction target message not found")
			return true
		} else {
			return zc.UserLogin.QueueRemoteEvent(&ReactionEvent{zc: zc, Reaction: evt, portal: part.Room}).Success
		}
	default:
		log.Debug().Type("event_type", evt).Msg("Unsupported event type")
		return true
	}
}

type ReactionEvent struct {
	zc     *ZulipClient
	portal networkid.PortalKey
	*events.Reaction
}

var (
	_ bridgev2.RemoteReaction       = (*ReactionEvent)(nil)
	_ bridgev2.RemoteReactionRemove = (*ReactionEvent)(nil)
)

func (r *ReactionEvent) GetType() bridgev2.RemoteEventType {
	if r.Op == "remove" {
		return bridgev2.RemoteEventReactionRemove
	}
	return bridgev2.RemoteEventReaction
}

func (r *ReactionEvent) GetPortalKey() networkid.PortalKey {
	return r.portal
}

func (r *ReactionEvent) AddLogContext(c zerolog.Context) zerolog.Context {
	return c
}

func (r *ReactionEvent) GetSender() bridgev2.EventSender {
	return r.zc.makeEventSender(r.UserID)
}

func (r *ReactionEvent) GetTargetMessage() networkid.MessageID {
	return zid.MakeMessageID(r.MessageID)
}

func (r *ReactionEvent) GetReactionEmoji() (emoji string, emojiID networkid.EmojiID) {
	switch r.ReactionType {
	case "unicode_emoji":
		emoji = zulipemoji.UnifiedToUnicode(r.EmojiCode)
	case "realm_emoji":
		// TODO custom emoji support
		emoji = r.EmojiName
	case "zulip_extra_emoji":
		// TODO custom emoji support
		emoji = r.EmojiName
	default:
		emoji = r.EmojiName
	}
	emojiID = networkid.EmojiID(r.EmojiCode)
	return
}

func (r *ReactionEvent) GetRemovedEmojiID() networkid.EmojiID {
	return networkid.EmojiID(r.EmojiCode)
}

func (zc *ZulipClient) makeTopicUpsert(name string, streamID int) bridgev2.RemoteMessage {
	return &simplevent.PreConvertedMessage{
		EventMeta: simplevent.EventMeta{
			Type: bridgev2.RemoteEventMessageUpsert,
			LogContext: func(c zerolog.Context) zerolog.Context {
				return c.Int("stream_id", streamID).Str("topic_name", name)
			},
			PortalKey:    zc.makeChannelPortalKey(streamID),
			CreatePortal: true,
		},
		ID: zid.MakeTopicMessageID(name),
		Data: &bridgev2.ConvertedMessage{
			Parts: []*bridgev2.ConvertedMessagePart{{
				Type: event.EventMessage,
				Content: &event.MessageEventContent{
					MsgType: event.MsgNotice,
					Body:    fmt.Sprintf("Topic created: %s", name),
				},
			}},
		},
	}
}

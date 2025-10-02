package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/bridgev2/simplevent"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func (zc *ZulipClient) handleZulipEvent(ctx context.Context, rawEvt events.Event) bool {
	log := zerolog.Ctx(ctx)
	switch evt := rawEvt.(type) {
	case *events.UserTopic:
		if evt.TopicName == "" {
			return true
		}
		return zc.UserLogin.QueueRemoteEvent(&TopicUpsert{Name: evt.TopicName, StreamID: evt.StreamID, c: zc}).Success
	case *events.Message:
		if evt.Message.StreamID != 0 && evt.Message.Subject != "" {
			if !zc.UserLogin.QueueRemoteEvent(&TopicUpsert{Name: evt.Message.Subject, StreamID: evt.Message.StreamID, c: zc}).Success {
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
			Data:               &evt.Message,
			ID:                 makeMessageID(evt.Message.ID),
			TransactionID:      networkid.TransactionID(evt.LocalID),
			ConvertMessageFunc: zc.convertZulipMessage,
		}).Success
	default:
		log.Debug().Type("event_type", evt).Msg("Unsupported event type")
		return true
	}
}

type TopicUpsert struct {
	Name     string
	StreamID int
	c        *ZulipClient
}

var (
	_ bridgev2.RemoteMessageUpsert            = (*TopicUpsert)(nil)
	_ bridgev2.RemoteEventThatMayCreatePortal = (*TopicUpsert)(nil)
)

func (t *TopicUpsert) GetType() bridgev2.RemoteEventType {
	return bridgev2.RemoteEventMessageUpsert
}

func (t *TopicUpsert) GetPortalKey() networkid.PortalKey {
	return t.c.makeChannelPortalKey(t.StreamID)
}

func (t *TopicUpsert) AddLogContext(c zerolog.Context) zerolog.Context {
	return c.Int("stream_id", t.StreamID).Str("topic_name", t.Name)
}

func (t *TopicUpsert) GetSender() bridgev2.EventSender {
	return bridgev2.EventSender{}
}

func (t *TopicUpsert) GetID() networkid.MessageID {
	return makeTopicMessageID(t.Name)
}

func (t *TopicUpsert) ShouldCreatePortal() bool {
	return true
}

func (t *TopicUpsert) ConvertMessage(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI) (*bridgev2.ConvertedMessage, error) {
	return &bridgev2.ConvertedMessage{
		Parts: []*bridgev2.ConvertedMessagePart{{
			Type: event.EventMessage,
			Content: &event.MessageEventContent{
				MsgType: event.MsgNotice,
				Body:    fmt.Sprintf("Topic created: %s", t.Name),
			},
		}},
	}, nil
}

func (t *TopicUpsert) HandleExisting(ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, existing []*database.Message) (bridgev2.UpsertResult, error) {
	// No-op, we just want the topic start to exist
	return bridgev2.UpsertResult{}, nil
}

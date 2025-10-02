package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2/status"

	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime"
	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func (zc *ZulipClient) pollQueue(ctx context.Context) {
	rtc := realtime.NewService(zc.Client)
	log := zc.UserLogin.Log.With().Str("component", "zulip poll").Logger()
	ctx = log.WithContext(ctx)
	stopChan := make(chan struct{})
	ctx, cancel := context.WithCancel(ctx)
	defer func() {
		cancel()
		close(stopChan)
		zc.UserLogin.Log.Debug().Msg("Polling stopped")
	}()
	if oldCancel := zc.stopPoll.Swap(&cancel); oldCancel != nil {
		(*oldCancel)()
	}
	zc.pollStopped.Store(&stopChan)
	queueID, lastEventID, err := zc.registerQueue(ctx, rtc)
	if err != nil {
		log.Err(err).Msg("Failed to register event queue")
		zc.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateUnknownError,
			Error:      "zulip-queue-register-error",
			Info: map[string]any{
				"go_error": err.Error(),
			},
		})
		return
	}
	// TODO only send this after first successful poll, except if we just registered
	zc.UserLogin.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
	for {
		resp, err := rtc.GetEventsEventQueue(ctx, queueID, realtime.LastEventID(lastEventID))
		if err != nil {
			// TODO re-register event queue if needed
			log.Err(err).Msg("Failed to poll event queue")
			zc.UserLogin.BridgeState.Send(status.BridgeState{
				StateEvent: status.StateTransientDisconnect,
				Error:      "zulip-event-poll-error",
				Info: map[string]any{
					"go_error": err.Error(),
				},
			})
			select {
			case <-time.After(10 * time.Second):
			case <-ctx.Done():
				return
			}
		} else {
			for _, evt := range resp.Events {
				ok := zc.handleZulipEvent(ctx, evt)
				if !ok {
					log.Warn().Int("event_id", evt.EventID()).Msg("Failed to handle event")
					break
				}
				lastEventID = evt.EventID()
			}
		}
	}
}

func (zc *ZulipClient) registerQueue(ctx context.Context, rtc *realtime.Service) (string, int, error) {
	meta := zc.UserLogin.Metadata.(*UserLoginMetadata)
	if meta.QueueID != "" && meta.LastEventID != 0 {
		return meta.QueueID, meta.LastEventID, nil
	}
	resp, err := rtc.RegisterEventQueue(
		ctx,
		realtime.EventTypes(
			events.AlertWordsType,
			events.AttachmentType,
			events.MessageType,
			events.PresenceType,
			events.RealmEmojiType,
			events.RealmUserType,
			events.SubmessageType,
			events.TypingType,
			events.UpdateMessageType,
		),
		realtime.ClientCapabilities(map[realtime.ClientCapability]bool{
			realtime.NotificationSettingsNull:   true,
			realtime.BulkMessageDeletion:        true,
			realtime.UserAvatarURLFieldOptional: true,
			realtime.StreamTypingNotifications:  true,
			realtime.UserSettingsObject:         true,
			realtime.LinkifierURLTemplate:       true,
			realtime.UserListIncomplete:         false,
			realtime.IncludeDeactivatedGroups:   false,
			realtime.ArchivedChannels:           false,
			realtime.EmptyTopicName:             true,
			realtime.SimplifiedPresenceEvents:   true,
		}),
		realtime.ClientGravatarEvent(true),
	)
	if err != nil {
		return "", 0, fmt.Errorf("failed to register event queue: %w", err)
	}
	zerolog.Ctx(ctx).Debug().Any("queue_register_resp", resp).Msg("Registered queue")
	meta.QueueID = resp.QueueID
	err = zc.UserLogin.Save(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to save queue ID: %w", err)
	}
	return resp.QueueID, resp.LastEventID, nil
}

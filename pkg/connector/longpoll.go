package connector

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"maunium.net/go/mautrix/bridgev2/status"

	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip"
	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime"
	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

/*func (zc *ZulipClient) syncChats(ctx context.Context) {
	resp, err := channels.NewService(zc.Client).GetSubscribedChannels(
		ctx, channels.IncludeSubscribersList(true),
	)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to get subscribed channels")
		return
	}
	for _, sub := range resp.Subscriptions {
		fmt.Println(sub)
	}
}*/

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
	zc.UserLogin.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnecting})

	var connectedSent bool
	meta := zc.UserLogin.Metadata.(*zid.UserLoginMetadata)
	for {
		if meta.QueueID == "" {
			err := zc.registerQueue(ctx, rtc)
			if err != nil {
				log.Err(err).Msg("Failed to register event queue")
				zc.UserLogin.BridgeState.Send(status.BridgeState{
					StateEvent: status.StateUnknownError,
					Error:      "zulip-queue-register-error",
					Info: map[string]any{
						"go_error": err.Error(),
					},
				})
				connectedSent = false
				// TODO retry on network errors, bad credentials on auth errors
				return
			}
			zc.UserLogin.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
			connectedSent = true
		}
		resp, err := rtc.GetEventsEventQueue(
			ctx, meta.QueueID, realtime.LastEventID(meta.LastEventID), realtime.DontBlock(!connectedSent),
		)
		if err != nil {
			log.Err(err).Msg("Failed to poll event queue")
			if zulip.IsCode(err, zulip.ErrBadEventQueueID) {
				meta.QueueID = ""
				err = zc.UserLogin.Save(ctx)
				if err != nil {
					log.Err(err).Msg("Failed to save cleared queue ID")
				}
				continue
			}
			zc.UserLogin.BridgeState.Send(status.BridgeState{
				StateEvent: status.StateTransientDisconnect,
				Error:      "zulip-event-poll-error",
				Info: map[string]any{
					"go_error": err.Error(),
				},
			})
			connectedSent = false
			select {
			case <-time.After(10 * time.Second):
			case <-ctx.Done():
				return
			}
			continue
		}
		if !connectedSent {
			zc.UserLogin.BridgeState.Send(status.BridgeState{StateEvent: status.StateConnected})
			connectedSent = true
		}
		for _, evt := range resp.Events {
			ok := zc.handleZulipEvent(ctx, evt)
			if !ok {
				log.Warn().Int("event_id", evt.EventID()).Msg("Failed to handle event")
				break
			}
			meta.LastEventID = evt.EventID()
		}
		if len(resp.Events) > 0 {
			err = zc.UserLogin.Save(ctx)
			if err != nil {
				log.Err(err).Msg("Failed to save last event ID")
			}
		}
	}
}

func (zc *ZulipClient) registerQueue(ctx context.Context, rtc *realtime.Service) error {
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
			events.DeleteMessageType,
			events.ReactionType,
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
		realtime.ApplyMarkdown(true),
	)
	if err != nil {
		zerolog.Ctx(ctx).Err(err).Msg("Failed to register event queue")
		zc.UserLogin.BridgeState.Send(status.BridgeState{
			StateEvent: status.StateUnknownError,
			Error:      "zulip-queue-register-error",
			Info: map[string]any{
				"go_error": err.Error(),
			},
		})
		return fmt.Errorf("failed to register event queue: %w", err)
	}
	zerolog.Ctx(ctx).Debug().Any("queue_register_resp", resp).Msg("Registered queue")
	meta := zc.UserLogin.Metadata.(*zid.UserLoginMetadata)
	meta.QueueID = resp.QueueID
	meta.LastEventID = resp.LastEventID
	return nil
}

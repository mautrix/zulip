package connector

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip/channels"
	"go.mau.fi/mautrix-zulip/pkg/zulip/users"
)

func (zc *ZulipClient) IsThisUser(ctx context.Context, userID networkid.UserID) bool {
	return zid.ParseUserID(userID) == zc.ownUserID
}

func (zc *ZulipClient) GetChatInfo(ctx context.Context, portal *bridgev2.Portal) (*bridgev2.ChatInfo, error) {
	streamID, userIDs, err := zid.ParsePortalID(portal.ID)
	if err != nil {
		return nil, err
	} else if userIDs != nil {
		return zc.wrapDMInfo(userIDs)
	} else {
		srv := channels.NewService(zc.Client)
		chat, err := srv.GetChannelByID(ctx, streamID)
		if err != nil {
			return nil, err
		}
		members, err := srv.GetChannelSubscribers(ctx, streamID)
		if err != nil {
			return nil, err
		}
		return zc.wrapChannelInfo(chat.Stream, members.Subscribers)
	}
}

func (zc *ZulipClient) wrapDMInfo(members []int) (*bridgev2.ChatInfo, error) {
	var otherUserID networkid.UserID
	portalType := database.RoomTypeGroupDM
	if len(members) == 1 {
		otherUserID = zid.MakeUserID(members[0])
		portalType = database.RoomTypeDM
	}
	memberMap := zc.makeMemberMap(members)
	memberMap[zid.MakeUserID(zc.ownUserID)] = bridgev2.ChatMember{
		EventSender: zc.makeEventSender(zc.ownUserID),
		Membership:  event.MembershipJoin,
	}
	return &bridgev2.ChatInfo{
		Members: &bridgev2.ChatMemberList{
			IsFull:           true,
			TotalMemberCount: len(members),
			MemberMap:        memberMap,
			OtherUserID:      otherUserID,
		},
		Type: &portalType,
	}, nil
}

func (zc *ZulipClient) makeMemberMap(members []int) map[networkid.UserID]bridgev2.ChatMember {
	memberMap := make(map[networkid.UserID]bridgev2.ChatMember, len(members))
	for _, m := range members {
		memberMap[zid.MakeUserID(m)] = bridgev2.ChatMember{
			EventSender: zc.makeEventSender(m),
			Membership:  event.MembershipJoin,
		}
	}
	return memberMap
}

func (zc *ZulipClient) wrapChannelInfo(channel channels.ChannelInfo, members []int) (*bridgev2.ChatInfo, error) {
	return &bridgev2.ChatInfo{
		Name:  &channel.Name,
		Topic: &channel.Description,
		Members: &bridgev2.ChatMemberList{
			IsFull:           members != nil,
			TotalMemberCount: len(members),
			MemberMap:        zc.makeMemberMap(members),
			PowerLevels:      nil, // TODO
		},
		Type: ptr.Ptr(database.RoomTypeDefault),
	}, nil
}

func (zc *ZulipClient) GetUserInfo(ctx context.Context, ghost *bridgev2.Ghost) (*bridgev2.UserInfo, error) {
	user, err := users.NewService(zc.Client).GetUser(ctx, zid.ParseUserID(ghost.ID))
	if err != nil {
		return nil, err
	}
	return wrapUserInfo(user.User)
}

var AvatarClient = &http.Client{
	Timeout: 20 * time.Second,
}

func wrapUserInfo(user users.UserData) (*bridgev2.UserInfo, error) {
	var identifiers []string
	if user.DeliveryEmail != "" {
		identifiers = []string{"mailto:" + user.DeliveryEmail}
	}
	return &bridgev2.UserInfo{
		Identifiers: identifiers,
		Name:        &user.FullName,
		Avatar:      wrapAvatar(user.AvatarVersion, user.AvatarURL, user.DeliveryEmail),
		IsBot:       &user.IsBot,
	}, nil
}

func wrapAvatar(version int, url, email string) *bridgev2.Avatar {
	if url == "" {
		if email == "" {
			return nil
		}
		emailHash := sha256.Sum256([]byte(email))
		url = fmt.Sprintf("https://www.gravatar.com/avatar/%x", emailHash[:])
	}
	return &bridgev2.Avatar{
		ID: networkid.AvatarID(strconv.Itoa(version)),
		Get: func(ctx context.Context) ([]byte, error) {
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("User-Agent", mautrix.DefaultUserAgent)
			resp, err := AvatarClient.Do(req)
			if err != nil {
				return nil, err
			} else if resp.StatusCode >= 300 {
				return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
			}
			return io.ReadAll(resp.Body)
		},
	}
}

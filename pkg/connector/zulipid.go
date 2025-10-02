package connector

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.mau.fi/util/exslices"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"

	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func makeMessageID(id int) networkid.MessageID {
	return networkid.MessageID(strconv.Itoa(id))
}

func makeTopicMessageID(topicName string) networkid.MessageID {
	return networkid.MessageID("topic:" + topicName)
}

func parseMessageID(id networkid.MessageID) (string, int) {
	if strings.HasPrefix(string(id), "topic:") {
		return strings.TrimPrefix(string(id), "topic:"), 0
	}
	n, _ := strconv.Atoi(string(id))
	return "", n
}

func parseUserLoginID(id networkid.UserLoginID) int {
	n, _ := strconv.Atoi(string(id))
	return n
}

func makeUserLoginID(id int) networkid.UserLoginID {
	return networkid.UserLoginID(strconv.Itoa(id))
}

func parseUserID(id networkid.UserID) int {
	n, _ := strconv.Atoi(string(id))
	return n
}

func makeUserID(id int) networkid.UserID {
	return networkid.UserID(strconv.Itoa(id))
}

func (zc *ZulipClient) makeEventSender(id int) bridgev2.EventSender {
	return bridgev2.EventSender{
		IsFromMe:    id == zc.ownUserID,
		SenderLogin: makeUserLoginID(id),
		Sender:      makeUserID(id),
	}
}

func makeChannelPortalID(streamID int) networkid.PortalID {
	return networkid.PortalID(fmt.Sprintf("stream:%d", streamID))
}

func makeDMPortalID(users []int) networkid.PortalID {
	slices.Sort(users)
	userStrings := exslices.CastFunc(users, func(from int) string {
		return strconv.Itoa(from)
	})
	return networkid.PortalID(fmt.Sprintf("dm:%s", strings.Join(userStrings, ",")))
}

func parsePortalID(portalID networkid.PortalID) (streamID int, userIDs []int, err error) {
	parts := strings.SplitN(string(portalID), ":", 2)
	if len(parts) != 2 {
		err = fmt.Errorf("invalid portal ID: %s", portalID)
		return
	}
	switch parts[0] {
	case "stream":
		streamID, err = strconv.Atoi(parts[1])
		if err != nil {
			err = fmt.Errorf("invalid stream portal ID: %s", portalID)
			return
		}
	case "dm":
		userParts := strings.Split(parts[1], ",")
		userIDs = make([]int, len(userParts))
		for i, up := range userParts {
			userIDs[i], err = strconv.Atoi(up)
			if err != nil {
				err = fmt.Errorf("invalid DM portal ID: %s", portalID)
				return
			}
		}
	default:
		err = fmt.Errorf("invalid portal ID prefix: %s", portalID)
	}
	return
}

func (zc *ZulipClient) makeChannelPortalKey(streamID int) (pk networkid.PortalKey) {
	if zc.UserLogin.Bridge.Config.SplitPortals {
		pk.Receiver = zc.UserLogin.ID
	}
	pk.ID = makeChannelPortalID(streamID)
	return pk
}

func (zc *ZulipClient) makePortalKey(message events.MessageData) (pk networkid.PortalKey) {
	if zc.UserLogin.Bridge.Config.SplitPortals || message.StreamID == 0 {
		pk.Receiver = zc.UserLogin.ID
	}
	if message.StreamID != 0 {
		pk.ID = makeChannelPortalID(message.StreamID)
	} else {
		pk.ID = makeDMPortalID(exslices.CastFuncFilter(message.DisplayRecipient.Users, func(from events.DisplayRecipientObject) (int, bool) {
			if from.ID == zc.ownUserID {
				return 0, false
			}
			return from.ID, true
		}))
	}
	return pk
}

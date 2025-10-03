package zid

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.mau.fi/util/exslices"
	"maunium.net/go/mautrix/bridgev2/networkid"
)

func MakeMessageID(id int) networkid.MessageID {
	return networkid.MessageID(strconv.Itoa(id))
}

func MakeTopicMessageID(topicName string) networkid.MessageID {
	return networkid.MessageID("topic:" + topicName)
}

func ParseMessageID(id networkid.MessageID) (string, int) {
	if strings.HasPrefix(string(id), "topic:") {
		return strings.TrimPrefix(string(id), "topic:"), 0
	}
	n, _ := strconv.Atoi(string(id))
	return "", n
}

func ParseUserLoginID(id networkid.UserLoginID) int {
	n, _ := strconv.Atoi(string(id))
	return n
}

func MakeUserLoginID(id int) networkid.UserLoginID {
	return networkid.UserLoginID(strconv.Itoa(id))
}

func ParseUserID(id networkid.UserID) int {
	n, _ := strconv.Atoi(string(id))
	return n
}

func MakeUserID(id int) networkid.UserID {
	return networkid.UserID(strconv.Itoa(id))
}

func MakeChannelPortalID(streamID int) networkid.PortalID {
	return networkid.PortalID(fmt.Sprintf("stream:%d", streamID))
}

func MakeDMPortalID(users []int) networkid.PortalID {
	slices.Sort(users)
	userStrings := exslices.CastFunc(users, func(from int) string {
		return strconv.Itoa(from)
	})
	return networkid.PortalID(fmt.Sprintf("dm:%s", strings.Join(userStrings, ",")))
}

func ParsePortalID(portalID networkid.PortalID) (streamID int, userIDs []int, err error) {
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

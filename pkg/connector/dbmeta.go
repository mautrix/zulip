package connector

import (
	"maunium.net/go/mautrix/bridgev2/database"
)

func (zc *ZulipConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Portal:   nil,
		Ghost:    nil,
		Message:  nil,
		Reaction: nil,
		UserLogin: func() any {
			return &UserLoginMetadata{}
		},
	}
}

type UserLoginMetadata struct {
	URL   string `json:"url"`
	Email string `json:"email"`
	Token string `json:"token"`

	QueueID     string `json:"queue_id,omitempty"`
	LastEventID int    `json:"last_event_id,omitempty"`
}

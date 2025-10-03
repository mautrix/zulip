package connector

import (
	"maunium.net/go/mautrix/bridgev2/database"

	"go.mau.fi/mautrix-zulip/pkg/zid"
)

func (zc *ZulipConnector) GetDBMetaTypes() database.MetaTypes {
	return database.MetaTypes{
		Portal:   nil,
		Ghost:    nil,
		Message:  nil,
		Reaction: nil,
		UserLogin: func() any {
			return &zid.UserLoginMetadata{}
		},
	}
}

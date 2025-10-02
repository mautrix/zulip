package main

import (
	"maunium.net/go/mautrix/bridgev2/matrix/mxmain"

	"go.mau.fi/mautrix-zulip/pkg/connector"
)

// Information to find out exactly which commit the bridge was built from.
// These are filled at build time with the -X linker flag.
var (
	Tag       = "unknown"
	Commit    = "unknown"
	BuildTime = "unknown"
)

func main() {
	m := mxmain.BridgeMain{
		Name:        "mautrix-zulip",
		Description: "A Matrix-Zulip bridge",
		URL:         "https://github.com/mautrix/zulip",
		Version:     "25.10",
		SemCalVer:   true,
		Connector:   &connector.ZulipConnector{},
	}
	m.InitVersion(Tag, Commit, BuildTime)
	m.Run()
}

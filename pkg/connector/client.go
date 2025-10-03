package connector

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	slogzerolog "github.com/samber/slog-zerolog/v2"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridgev2"

	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip"
)

type ZulipClient struct {
	Main      *ZulipConnector
	UserLogin *bridgev2.UserLogin
	Client    *zulip.Client

	stopPoll    atomic.Pointer[context.CancelFunc]
	pollStopped atomic.Pointer[chan struct{}]
	ownUserID   int
}

func (zc *ZulipConnector) LoadUserLogin(ctx context.Context, login *bridgev2.UserLogin) error {
	meta := login.Metadata.(*zid.UserLoginMetadata)
	httpClient := &http.Client{Timeout: 180 * time.Second}
	zulipLog := login.Log.With().Str("component", "zulip").Logger()
	cli, err := zulip.NewClient(
		zulip.Credentials(meta.URL, meta.Email, meta.Token),
		zulip.WithCustomUserAgent(mautrix.DefaultUserAgent),
		zulip.WithLogger(slog.New(slogzerolog.Option{Logger: &zulipLog}.NewZerologHandler())),
		zulip.WithHTTPClient(httpClient),
	)
	if err != nil {
		return err
	}
	login.Client = &ZulipClient{
		Main:      zc,
		Client:    cli,
		UserLogin: login,
		ownUserID: zid.ParseUserLoginID(login.ID),
	}
	return nil
}

func (zc *ZulipClient) Connect(ctx context.Context) {
	go zc.pollQueue(ctx)
}

func (zc *ZulipClient) Disconnect() {
	pollStop := zc.pollStopped.Load()
	if cancel := zc.stopPoll.Swap(nil); cancel != nil {
		(*cancel)()
	}
	if pollStop != nil {
		select {
		case <-*pollStop:
		case <-time.After(5 * time.Second):
			zc.UserLogin.Log.Warn().Msg("Timed out waiting for poll to stop")
		}
	}
}

func (zc *ZulipClient) IsLoggedIn() bool {
	return zc.Client != nil
}

func (zc *ZulipClient) LogoutRemote(ctx context.Context) {
	// We don't own the api token, so don't do anything
}

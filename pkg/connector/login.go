package connector

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/database"
	"maunium.net/go/mautrix/bridgev2/status"

	"go.mau.fi/mautrix-zulip/pkg/zulip"
	"go.mau.fi/mautrix-zulip/pkg/zulip/users"
)

var loginFlows = []bridgev2.LoginFlow{{
	Name:        "API token",
	Description: "Login with your Zulip email and API token.",
	ID:          "apitoken",
}}

func (zc *ZulipConnector) GetLoginFlows() []bridgev2.LoginFlow {
	return loginFlows
}

func (zc *ZulipConnector) CreateLogin(ctx context.Context, user *bridgev2.User, flowID string) (bridgev2.LoginProcess, error) {
	if flowID != loginFlows[0].ID {
		return nil, fmt.Errorf("unknown flow ID: %s", flowID)
	}
	return &ZulipLogin{User: user, Main: zc}, nil
}

type ZulipLogin struct {
	User *bridgev2.User
	Main *ZulipConnector
}

var _ bridgev2.LoginProcessUserInput = (*ZulipLogin)(nil)

func (zl *ZulipLogin) Start(ctx context.Context) (*bridgev2.LoginStep, error) {
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeUserInput,
		StepID:       "fi.mau.zulip.apitoken",
		Instructions: "",
		UserInputParams: &bridgev2.LoginUserInputParams{
			Fields: []bridgev2.LoginInputDataField{{
				Type: bridgev2.LoginInputFieldTypeURL,
				ID:   "url",
				Name: "Zulip server URL",
			}, {
				Type: bridgev2.LoginInputFieldTypeEmail,
				ID:   "email",
				Name: "Email",
			}, {
				Type:    bridgev2.LoginInputFieldTypeToken,
				ID:      "token",
				Name:    "API key",
				Pattern: "^[A-Za-z0-9]{32}$",
			}},
		},
	}, nil
}

func (zl *ZulipLogin) Cancel() {}

func (zl *ZulipLogin) SubmitUserInput(ctx context.Context, input map[string]string) (*bridgev2.LoginStep, error) {
	meta := &UserLoginMetadata{
		URL:   input["url"],
		Email: input["email"],
		Token: input["token"],
	}
	cli, err := zulip.NewClient(
		zulip.Credentials(meta.URL, meta.Email, meta.Token),
		zulip.WithCustomUserAgent(mautrix.DefaultUserAgent+" (login)"),
	)
	if err != nil {
		return nil, err
	}
	me, err := users.NewService(cli).GetUserMe(ctx)
	if err != nil {
		return nil, err
	}
	ul, err := zl.User.NewLogin(ctx, &database.UserLogin{
		ID:         makeUserLoginID(me.UserID),
		RemoteName: me.Email,
		RemoteProfile: status.RemoteProfile{
			Email: me.Email,
			Name:  me.FullName,
			// TODO reupload avatar
			Avatar: "",
		},
		Metadata: meta,
	}, &bridgev2.NewLoginParams{})
	if err != nil {
		return nil, err
	}
	go ul.Client.Connect(ul.Log.WithContext(zl.Main.Bridge.BackgroundCtx))
	return &bridgev2.LoginStep{
		Type:         bridgev2.LoginStepTypeComplete,
		StepID:       "fi.mau.zulip.complete",
		Instructions: fmt.Sprintf("Successfully logged in as %s", me.FullName),
	}, nil
}

package connector

import (
	"go.mau.fi/util/l10n"
)

var L = l10n.NewNamespace("mautrix-zulip")

var (
	msgFlowAPITokenName         = L.Define("login.flow.apitoken.name", "API token")
	msgFlowAPITokenDescription  = L.Define("login.flow.apitoken.description", "Login with your Zulip email and API token.")
	msgFieldURLName             = L.Define("login.field.url.name", "Zulip server URL")
	msgFieldEmailName           = L.Define("login.field.email.name", "Email")
	msgFieldTokenName           = L.Define("login.field.token.name", "API key")
	msgStepCompleteInstructions = L.Define(
		"login.step.complete.instructions", "Successfully logged in as {name}",
		l10n.WithComment("name is the full name of the Zulip user"),
	)
)

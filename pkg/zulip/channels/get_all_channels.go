package channels

import (
	"context"
	"encoding/json"
	"net/http"

	"go.mau.fi/mautrix-zulip/pkg/zulip"
)

type GetAllChannelsResponse struct {
	zulip.APIResponseBase
	getAllChannelsResponseData
}

type getAllChannelsResponseData struct {
	Streams []ChannelInfo `json:"streams"`
}

func (g *GetAllChannelsResponse) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &g.APIResponseBase); err != nil {
		return err
	}

	if err := json.Unmarshal(b, &g.getAllChannelsResponseData); err != nil {
		return err
	}

	return nil
}

// GetAllChannels Get all channels that the user has access to.
func (svc *Service) GetAllChannels(ctx context.Context) (*GetAllChannelsResponse, error) {
	const (
		method = http.MethodGet
		path   = "/api/v1/streams"
	)

	resp := GetAllChannelsResponse{}
	if err := svc.client.DoRequest(ctx, method, path, nil, &resp); err != nil {
		return nil, err
	}

	return &resp, nil
}

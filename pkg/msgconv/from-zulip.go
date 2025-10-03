package msgconv

import (
	"context"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/rs/zerolog"
	"go.mau.fi/util/ptr"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/bridgev2/networkid"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/format"
	"maunium.net/go/mautrix/id"

	"go.mau.fi/mautrix-zulip/pkg/msgconv/zuliphtml"
	"go.mau.fi/mautrix-zulip/pkg/zid"
	"go.mau.fi/mautrix-zulip/pkg/zulip/realtime/events"
)

func ToMatrix(
	ctx context.Context, portal *bridgev2.Portal, intent bridgev2.MatrixAPI, source *bridgev2.UserLogin, data *events.MessageData,
) (*bridgev2.ConvertedMessage, error) {
	var threadRootID *networkid.MessageID
	if data.Subject != "" {
		threadRootID = ptr.Ptr(zid.MakeTopicMessageID(data.Subject))
	}
	meta := source.Metadata.(*zid.UserLoginMetadata)
	html, attachments, mentions, err := zuliphtml.Parse(ctx, portal.Bridge, meta.URL, data.Content)
	if err != nil {
		return nil, err
	}
	content := format.HTMLToContent(html)
	content.Mentions = mentions
	textPart := &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: &content,
	}
	hasText := textPart.Content.Body != "" || textPart.Content.FormattedBody != ""
	var parts []*bridgev2.ConvertedMessagePart
	if len(attachments) == 0 {
		parts = []*bridgev2.ConvertedMessagePart{textPart}
	} else if len(attachments) == 1 && hasText {
		mediaPart := attachmentToMatrix(ctx, portal.MXID, intent, meta, attachments[0])
		parts = []*bridgev2.ConvertedMessagePart{bridgev2.MergeCaption(textPart, mediaPart)}
	} else {
		parts = make([]*bridgev2.ConvertedMessagePart, 0, len(attachments)+1)
		if hasText {
			parts = append(parts, textPart)
		}
		for _, att := range attachments {
			parts = append(parts, attachmentToMatrix(ctx, portal.MXID, intent, meta, att))
		}
	}
	return &bridgev2.ConvertedMessage{
		ThreadRoot: threadRootID,
		Parts:      parts,
	}, nil
}

var MediaClient = &http.Client{
	Timeout: 60 * time.Second,
}

func attachmentToMatrix(
	ctx context.Context, roomID id.RoomID, intent bridgev2.MatrixAPI, meta *zid.UserLoginMetadata,
	attachment zuliphtml.Attachment,
) *bridgev2.ConvertedMessagePart {
	part, err := attachmentToMatrixWithErrors(ctx, roomID, intent, meta, attachment)
	if err != nil {
		return &bridgev2.ConvertedMessagePart{
			Type: event.EventMessage,
			Content: &event.MessageEventContent{
				MsgType: event.MsgNotice,
				Body:    fmt.Sprintf("Failed to fetch attachment: %v", err),
			},
		}
	}
	return part
}

func attachmentToMatrixWithErrors(
	ctx context.Context, roomID id.RoomID, intent bridgev2.MatrixAPI, meta *zid.UserLoginMetadata,
	attachment zuliphtml.Attachment,
) (*bridgev2.ConvertedMessagePart, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, attachment.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare request: %w", err)
	}
	req.Header.Set("User-Agent", mautrix.DefaultUserAgent)
	req.Header.Set("Accept", "*/*")
	if strings.HasPrefix(attachment.URL, meta.URL) {
		req.SetBasicAuth(meta.Email, meta.Token)
	}
	content := &event.MessageEventContent{
		MsgType: attachment.MsgType,
		Body:    attachment.FileName,
		Info:    &event.FileInfo{},
	}
	resp, err := MediaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}
	content.Info.MimeType = resp.Header.Get("Content-Type")
	content.URL, content.File, err = intent.UploadMediaStream(ctx, roomID, -1, true, func(file io.Writer) (*bridgev2.FileStreamResult, error) {
		n, err := io.Copy(file, resp.Body)
		if err != nil {
			return nil, err
		}
		realFile := file.(*os.File)
		// TODO this is probably unnecessary
		if content.Info.MimeType == "" {
			_, err = realFile.Seek(0, io.SeekStart)
			if err != nil {
				return nil, fmt.Errorf("failed to seek to start: %w", err)
			}
			mime, err := mimetype.DetectReader(realFile)
			if err != nil {
				return nil, fmt.Errorf("failed to detect mime type: %w", err)
			}
			content.Info.MimeType = mime.String()
		}
		if attachment.MsgType == event.MsgImage {
			_, err = realFile.Seek(0, io.SeekStart)
			if err != nil {
				return nil, fmt.Errorf("failed to seek to start: %w", err)
			}
			cfg, _, err := image.DecodeConfig(realFile)
			if err != nil {
				zerolog.Ctx(ctx).Warn().Err(err).Msg("Failed to decode image config")
			} else {
				content.Info.Width = cfg.Width
				content.Info.Height = cfg.Height
			}
		}
		content.Info.Size = int(n)
		return &bridgev2.FileStreamResult{
			FileName: attachment.FileName,
			MimeType: content.Info.MimeType,
		}, nil
	})
	if err != nil {
		return nil, err
	}
	return &bridgev2.ConvertedMessagePart{
		Type:    event.EventMessage,
		Content: content,
	}, nil
}

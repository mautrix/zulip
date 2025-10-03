package zuliphtml

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"maunium.net/go/mautrix/bridgev2"
	"maunium.net/go/mautrix/event"

	"go.mau.fi/mautrix-zulip/pkg/msgconv/zulipemoji"
	"go.mau.fi/mautrix-zulip/pkg/zid"
)

func Parse(
	ctx context.Context, bridge *bridgev2.Bridge, baseURL, inputHTML string,
) (outputHTML string, attachments []Attachment, mentions *event.Mentions, err error) {
	parser := &zulipHTMLParser{
		ctx:     ctx,
		br:      bridge,
		baseURL: baseURL,
	}
	err = parser.Parse(inputHTML)
	return parser.output, parser.attachments, &parser.mentions, err
}

type Attachment struct {
	URL      string
	MsgType  event.MessageType
	FileName string
}

type zulipHTMLParser struct {
	ctx         context.Context
	br          *bridgev2.Bridge
	baseURL     string
	output      string
	attachments []Attachment
	mentions    event.Mentions
}

func (zhp *zulipHTMLParser) Parse(input string) error {
	node, err := html.Parse(strings.NewReader(input))
	if err != nil {
		return err
	}
	bodyNode := findChild(node, atom.Body, "")
	if bodyNode != nil {
		node = bodyNode.FirstChild
	}
	if node.NextSibling == nil && node.DataAtom == atom.P {
		node = node.FirstChild
	}
	node = buildWithChild(&html.Node{
		Type:       html.DocumentNode,
		FirstChild: node,
	})
	err = zhp.processChildren(node)
	if err != nil {
		return err
	}
	var buf strings.Builder
	err = html.Render(&buf, node)
	if err != nil {
		return err
	}
	zhp.output = buf.String()
	return nil
}

func (zhp *zulipHTMLParser) processChildren(node *html.Node) error {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		err := zhp.processSingleNode(child)
		if err != nil {
			return err
		}
	}
	return nil
}

func (zhp *zulipHTMLParser) processSingleNode(node *html.Node) error {
	if node.Type != html.ElementNode {
		return nil
	}
	classes := getClass(node.Attr)
	handler, ok := handlers[node.DataAtom][firstClass(classes)]
	if ok {
		handled, err := handler(zhp, node)
		if err != nil {
			return err
		} else if handled {
			return nil
		}
	}
	switch node.DataAtom {
	case atom.A:
		hrefIdx := getAttributeIndex(node.Attr, "href")
		if hrefIdx >= 0 {
			node.Attr[hrefIdx].Val = zhp.makeAbsoluteURL(node.Attr[hrefIdx].Val)
		}
	}
	return zhp.processChildren(node)
}

var handlers = map[atom.Atom]map[string]func(*zulipHTMLParser, *html.Node) (bool, error){
	atom.A: {
		"stream":       (*zulipHTMLParser).processChannelLink,
		"stream-topic": (*zulipHTMLParser).processTopicLink,
		"message-link": (*zulipHTMLParser).processMessageLink,
	},
	atom.Div: {
		"codehilite":           (*zulipHTMLParser).processCodeBlock,
		"message_inline_image": (*zulipHTMLParser).processInlineMedia,
		"spoiler-block":        (*zulipHTMLParser).processSpoilerBlock,
	},
	atom.Span: {
		"emoji":         (*zulipHTMLParser).processEmoji,
		"katex-display": (*zulipHTMLParser).processKatex,
		"user-mention":  (*zulipHTMLParser).processUserMention,
		"topic-mention": (*zulipHTMLParser).processTopicMention,
	},
	atom.Img: {
		"emoji": (*zulipHTMLParser).processCustomEmoji,
	},
	atom.Audio: {
		"": (*zulipHTMLParser).processInlineAudio,
	},
}

func (zhp *zulipHTMLParser) processChannelLink(node *html.Node) (bool, error) {
	return false, nil
}

func (zhp *zulipHTMLParser) processTopicLink(node *html.Node) (bool, error) {
	return false, nil
}

func (zhp *zulipHTMLParser) processMessageLink(node *html.Node) (bool, error) {
	return false, nil
}

func (zhp *zulipHTMLParser) processCodeBlock(node *html.Node) (bool, error) {
	codeBlockLanguage, _ := getAttribute(node.Attr, "data-code-language")
	codeBlock := nodeText(node)
	var codeAttr []html.Attribute
	if codeBlockLanguage != "" {
		codeAttr = []html.Attribute{{
			Key: "class",
			Val: fmt.Sprintf("language-%s", strings.ToLower(codeBlockLanguage)),
		}}
	}
	*node = rebuildNode(node, &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Pre,
		FirstChild: buildWithChild(&html.Node{
			Type:     html.ElementNode,
			DataAtom: atom.Code,
			Attr:     codeAttr,
			FirstChild: &html.Node{
				Type: html.TextNode,
				Data: codeBlock,
			},
		}),
	})
	return true, nil
}

func (zhp *zulipHTMLParser) processInlineMedia(node *html.Node) (bool, error) {
	mediaLink := findChild(node, atom.A, "")
	if mediaLink == nil {
		return false, nil
	}
	href, ok := getAttribute(mediaLink.Attr, "href")
	if !ok {
		return false, nil
	}
	msgType := event.MsgImage
	if findChild(mediaLink, atom.Video, "") != nil {
		msgType = event.MsgVideo
	}
	title, _ := getAttribute(mediaLink.Attr, "title")
	if title == "" {
		parts := strings.SplitN(href, "?", 2)
		title = path.Base(parts[0])
	}
	zhp.attachments = append(zhp.attachments, Attachment{
		URL:      zhp.makeAbsoluteURL(href),
		MsgType:  msgType,
		FileName: title,
	})
	node.Parent.RemoveChild(node)
	return true, nil
}

func (zhp *zulipHTMLParser) makeAbsoluteURL(href string) string {
	if strings.HasPrefix(href, "/") && !strings.HasPrefix(href, "//") {
		return zhp.baseURL + href
	}
	return href
}

func (zhp *zulipHTMLParser) processInlineAudio(node *html.Node) (bool, error) {
	href, ok := getAttribute(node.Attr, "src")
	if !ok {
		return false, nil
	}
	title, _ := getAttribute(node.Attr, "title")
	zhp.attachments = append(zhp.attachments, Attachment{
		URL:      zhp.makeAbsoluteURL(href),
		MsgType:  event.MsgAudio,
		FileName: title,
	})
	node.Parent.RemoveChild(node)
	return true, nil
}

func (zhp *zulipHTMLParser) processSpoilerBlock(node *html.Node) (bool, error) {
	header := findChild(node, atom.Div, "spoiler-header")
	content := findChild(node, atom.Div, "spoiler-content")
	if header == nil || content == nil {
		return false, nil
	}
	reason := nodeText(header)
	*node = rebuildNode(node, &html.Node{
		Type:       html.ElementNode,
		DataAtom:   atom.Span,
		Attr:       []html.Attribute{{Key: "data-mx-spoiler", Val: reason}},
		FirstChild: content.FirstChild,
		LastChild:  content.LastChild,
	})
	return true, nil
}

func (zhp *zulipHTMLParser) processCustomEmoji(node *html.Node) (bool, error) {
	return false, nil
}

func (zhp *zulipHTMLParser) processEmoji(node *html.Node) (bool, error) {
	classes := getClass(node.Attr)
	for _, c := range classes {
		if strings.HasPrefix(c, "emoji-") {
			*node = rebuildNode(node, &html.Node{
				Type: html.TextNode,
				Data: zulipemoji.UnifiedToUnicode(c),
			})
			return true, nil
		}
	}
	return false, nil
}

func (zhp *zulipHTMLParser) processKatex(node *html.Node) (bool, error) {
	annotation := findChild(node, atom.Annotation, "")
	if annotation == nil {
		return false, nil
	}
	latexSource := nodeText(annotation)
	*node = rebuildNode(node, &html.Node{
		Type:     html.ElementNode,
		DataAtom: atom.Div,
		FirstChild: &html.Node{
			Type: html.TextNode,
			Data: latexSource,
		},
		Attr: []html.Attribute{{
			Key: "data-mx-maths",
			Val: latexSource,
		}},
	})
	return true, nil
}

func (zhp *zulipHTMLParser) processUserMention(node *html.Node) (bool, error) {
	userIDStr, ok := getAttribute(node.Attr, "data-user-id")
	if !ok {
		return false, nil
	}
	if userIDStr == "*" {
		*node = rebuildNode(node, &html.Node{
			Type: html.TextNode,
			Data: "@room",
		})
		zhp.mentions.Room = true
	} else if userID, err := strconv.Atoi(userIDStr); err == nil {
		var ghost *bridgev2.Ghost
		var userLogin *bridgev2.UserLogin
		ghost, err = zhp.br.GetGhostByID(zhp.ctx, zid.MakeUserID(userID))
		if err != nil {
			return false, fmt.Errorf("failed to get ghost of mentioned user %d: %w", userID, err)
		}
		mxid := ghost.Intent.GetMXID()
		userLogin, err = zhp.br.GetExistingUserLoginByID(zhp.ctx, zid.MakeUserLoginID(userID))
		if err != nil {
			return false, fmt.Errorf("failed to get user login of mentioned user %d: %w", userID, err)
		} else if userLogin != nil {
			mxid = userLogin.UserMXID
		}

		if !slices.Contains(getClass(node.Attr), "silent") {
			zhp.mentions.Add(mxid)
		}
		node.Data = "a"
		node.DataAtom = atom.A
		node.Attr = []html.Attribute{{
			Key: "href",
			Val: mxid.URI().MatrixToURL(),
		}}
	} else {
		return false, nil
	}
	return true, nil
}

func (zhp *zulipHTMLParser) processTopicMention(node *html.Node) (bool, error) {
	return false, nil
}

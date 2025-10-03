package zuliphtml

import (
	"slices"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func getAttribute(attrs []html.Attribute, key string) (string, bool) {
	idx := getAttributeIndex(attrs, key)
	if idx == -1 {
		return "", false
	}
	return attrs[idx].Val, true
}

func getAttributeIndex(attrs []html.Attribute, key string) (idx int) {
	idx = -1
	for i, attr := range attrs {
		if attr.Key == key {
			idx = i
			return
		}
	}
	return
}

func getClass(attrs []html.Attribute) []string {
	attr, ok := getAttribute(attrs, "class")
	if !ok {
		return nil
	}
	return strings.Fields(attr)
}

func firstClass(classes []string) string {
	if len(classes) == 0 {
		return ""
	}
	return classes[0]
}

func nodeTextBuf(node *html.Node, buf *strings.Builder) {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode {
			nodeTextBuf(child, buf)
		} else if child.Type == html.TextNode {
			buf.WriteString(child.Data)
		}
	}
}

func nodeText(node *html.Node) string {
	var buf strings.Builder
	nodeTextBuf(node, &buf)
	return buf.String()
}

func buildWithChild(node *html.Node) *html.Node {
	var lastSibling *html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		child.Parent = node
		lastSibling = child
	}
	if node.LastChild == nil {
		node.LastChild = lastSibling
	}
	if node.Data == "" && node.DataAtom != 0 {
		node.Data = node.DataAtom.String()
	}
	return node
}

func rebuildNode(origNode, newContents *html.Node) html.Node {
	if newContents.FirstChild != nil {
		buildWithChild(newContents)
	}
	node := *origNode
	node.Type = newContents.Type
	node.DataAtom = newContents.DataAtom
	node.Data = newContents.Data
	node.Attr = newContents.Attr
	node.FirstChild = newContents.FirstChild
	node.LastChild = newContents.LastChild
	node.Namespace = newContents.Namespace
	return node
}

func findChild(node *html.Node, tag atom.Atom, class string) *html.Node {
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.ElementNode {
			// skip
		} else if child.DataAtom == tag && (class == "" || slices.Contains(getClass(child.Attr), class)) {
			return child
		} else if found := findChild(child, tag, class); found != nil {
			return found
		}
	}
	return nil
}

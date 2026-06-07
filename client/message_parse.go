package client

import (
	"strings"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/client/extensions"
)

// ParseMode constants mirror Telethon's parse-mode names.
const (
	ParseModeHTML     = "html"
	ParseModeMarkdown = "md"
	ParseModeNone     = ""
)

// SanitizeParseMode normalises a parse-mode string to one of the ParseMode*
// constants. It accepts common aliases ("markdown", "htm") and returns
// ParseModeNone for unknown/empty inputs.
func SanitizeParseMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "html", "htm":
		return ParseModeHTML
	case "md", "markdown":
		return ParseModeMarkdown
	default:
		return ParseModeNone
	}
}

// ParseText parses text with entities using the specified parse mode.
// Returns plain text and a slice of MessageEntityClass.
// An empty/unknown parseMode returns the text unchanged with no entities.
//
// HTML mode is an exact port of telethon/extensions/html.py parse().
// Markdown mode is an exact port of telethon/extensions/markdown.py parse().
func ParseText(text, parseMode string) (string, []tg.MessageEntityClass, error) {
	switch SanitizeParseMode(parseMode) {
	case ParseModeHTML:
		return extensions.ParseHTML(text)
	case ParseModeMarkdown:
		return extensions.ParseMarkdown(text)
	default:
		return text, nil, nil
	}
}

// UnparseText converts a plain message and its entity list back to a
// formatted string for the given parse mode.
//
// HTML mode is an exact port of telethon/extensions/html.py unparse().
// Markdown mode is an exact port of telethon/extensions/markdown.py unparse().
func UnparseText(text string, entities []tg.MessageEntityClass, parseMode string) string {
	switch SanitizeParseMode(parseMode) {
	case ParseModeHTML:
		return extensions.UnparseHTML(text, entities)
	case ParseModeMarkdown:
		return extensions.UnparseMarkdown(text, entities)
	default:
		return text
	}
}

// HTMLToEntities is an alias for extensions.ParseHTML kept for back-compat.
func HTMLToEntities(html string) (string, []tg.MessageEntityClass, error) {
	return extensions.ParseHTML(html)
}

// EntitiesToHTML is an alias for extensions.UnparseHTML kept for back-compat.
func EntitiesToHTML(text string, entities []tg.MessageEntityClass) string {
	return extensions.UnparseHTML(text, entities)
}

// MarkdownToEntities is an alias for extensions.ParseMarkdown kept for back-compat.
func MarkdownToEntities(md string) (string, []tg.MessageEntityClass, error) {
	return extensions.ParseMarkdown(md)
}

// EntitiesToMarkdown is an alias for extensions.UnparseMarkdown kept for back-compat.
func EntitiesToMarkdown(text string, entities []tg.MessageEntityClass) string {
	return extensions.UnparseMarkdown(text, entities)
}

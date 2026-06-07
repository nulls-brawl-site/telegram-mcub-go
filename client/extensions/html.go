// Package extensions provides HTML and Markdown parsers that are exact Go
// ports of Telethon's telethon/extensions/html.py and markdown.py.
//
// Telegram entity offsets/lengths are measured in UTF-16 code units.  All
// parsers in this package track positions in UTF-16 space so that emoji
// (which occupy 2 UTF-16 code units but 1 rune) are counted correctly.
package extensions

import (
	"fmt"
	gohtml "html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/helpers"
)

// ---------------------------------------------------------------------------
// Pre-processing helpers (port of _escape_invalid_tag_syntax)
// ---------------------------------------------------------------------------

var (
	// Matches any angle-bracket construct that looks like an HTML tag.
	angleTagRE = regexp.MustCompile(`<\s*/?\s*([^\s<>/]+)(?:\s[^<>]*?)?\s*/?\s*>`)
	// A tag name is valid only if it matches this pattern.
	validTagNameRE = regexp.MustCompile(`^[A-Za-z][-.A-Za-z0-9:_]*$`)
)

// escapeInvalidTagSyntax is a port of telethon/extensions/html.py
// _escape_invalid_tag_syntax().  It escapes angle-bracket constructs whose
// tag name does not match the valid pattern (e.g. "<3 hearts>").
func escapeInvalidTagSyntax(text string) string {
	return angleTagRE.ReplaceAllStringFunc(text, func(match string) string {
		sub := angleTagRE.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		if validTagNameRE.MatchString(sub[1]) {
			return match
		}
		// Escape < and > to &lt; and &gt;
		s := strings.ReplaceAll(match, "&", "&amp;")
		s = strings.ReplaceAll(s, "<", "&lt;")
		s = strings.ReplaceAll(s, ">", "&gt;")
		return s
	})
}

// ---------------------------------------------------------------------------
// Supported tags (mirrors _SUPPORTED_TAGS in html.py)
// ---------------------------------------------------------------------------

var supportedTags = map[string]bool{
	"strong": true, "b": true,
	"em": true, "i": true,
	"u": true,
	"del": true, "s": true,
	"blockquote": true,
	"code":       true,
	"mono":       true,
	"pre":        true,
	"tg-emoji":   true,
	"tg-spoiler": true,
	"a":          true,
	"emoji":      true,
}

// literalTagSentinel is used in openTagsMeta to mark a tag that should be
// output literally (analogous to _LITERAL_TAG in html.py).
type literalTagSentinel struct{}

var literalTag = &literalTagSentinel{}

// ---------------------------------------------------------------------------
// Custom lightweight HTML tokenizer
// ---------------------------------------------------------------------------
// We use our own tokenizer rather than golang.org/x/net/html to match
// Python's HTMLParser behaviour exactly: case-insensitive tag names,
// lenient attribute parsing, and verbatim preservation of unrecognised tags.

type tokType int

const (
	tokText     tokType = iota
	tokStart            // opening tag
	tokEnd              // closing tag
	tokSelfClose        // self-closing tag (treated as start for our purposes)
)

type htmlTok struct {
	typ   tokType
	tag   string            // lower-cased tag name
	attrs map[string]string // attribute name → value (decoded)
	// attrRaw preserves original attribute list order (for boolean attributes).
	attrRaw []htmlAttr
	raw     string // original tag text (e.g. "<br />"), used for literal output
	data    string // decoded text data (only for tokText)
}

type htmlAttr struct {
	key string
	val string
	// isBoolean is true for attributes without a value (e.g. "expandable").
	isBoolean bool
}

// tokenizeHTML produces a slice of tokens from raw HTML.
// It is intentionally lenient: invalid sequences are returned as text tokens.
func tokenizeHTML(input string) []htmlTok {
	var toks []htmlTok
	i := 0
	n := len(input)

	for i < n {
		if input[i] != '<' {
			// Collect text until the next '<'.
			j := i
			for j < n && input[j] != '<' {
				j++
			}
			toks = append(toks, htmlTok{typ: tokText, data: gohtml.UnescapeString(input[i:j])})
			i = j
			continue
		}

		// We're at '<'.  Find the matching '>'.
		tagEnd := findTagClose(input, i+1)
		if tagEnd < 0 {
			// No closing '>': treat rest as text.
			toks = append(toks, htmlTok{typ: tokText, data: gohtml.UnescapeString(input[i:])})
			break
		}

		raw := input[i : tagEnd+1]
		inner := input[i+1 : tagEnd] // content between < and >
		i = tagEnd + 1

		// Comments / CDATA / DOCTYPE — skip silently.
		if strings.HasPrefix(inner, "!") || strings.HasPrefix(inner, "?") {
			continue
		}

		isEnd := false
		if strings.HasPrefix(inner, "/") {
			isEnd = true
			inner = inner[1:]
		}

		inner = strings.TrimSpace(inner)
		// Handle self-closing "/>".
		isSelfClose := strings.HasSuffix(inner, "/")
		if isSelfClose {
			inner = strings.TrimSuffix(inner, "/")
			inner = strings.TrimSpace(inner)
		}

		if inner == "" {
			// Just "<>" or "</>" — treat as text.
			toks = append(toks, htmlTok{typ: tokText, data: raw})
			continue
		}

		// Split into tag name and the rest.
		tagName, rest := splitTagName(inner)
		tagName = strings.ToLower(tagName)

		if isEnd {
			toks = append(toks, htmlTok{typ: tokEnd, tag: tagName, raw: raw})
		} else {
			attrRaw, attrMap := parseTagAttrs(rest)
			typ := tokStart
			if isSelfClose {
				typ = tokSelfClose
			}
			toks = append(toks, htmlTok{
				typ: typ, tag: tagName,
				attrs: attrMap, attrRaw: attrRaw,
				raw: raw,
			})
		}
	}

	return toks
}

// findTagClose finds the index of the '>' that closes the tag starting at
// position start (which is the character after '<').  It skips '>' that
// appear inside quoted attribute values.
func findTagClose(s string, start int) int {
	i := start
	n := len(s)
	for i < n {
		switch s[i] {
		case '>':
			return i
		case '"':
			i++
			for i < n && s[i] != '"' {
				i++
			}
		case '\'':
			i++
			for i < n && s[i] != '\'' {
				i++
			}
		}
		i++
	}
	return -1
}

// splitTagName splits "tagname rest..." into (tagname, rest).
func splitTagName(s string) (name, rest string) {
	i := 0
	for i < len(s) && s[i] != ' ' && s[i] != '\t' && s[i] != '\n' && s[i] != '\r' {
		i++
	}
	return s[:i], s[i:]
}

// parseTagAttrs parses the attribute portion of a tag and returns a list
// (preserving order) and a map.
func parseTagAttrs(s string) ([]htmlAttr, map[string]string) {
	s = strings.TrimSpace(s)
	var raw []htmlAttr
	m := make(map[string]string)

	for s != "" {
		s = strings.TrimSpace(s)
		if s == "" || s == "/" {
			break
		}
		// Find end of key.
		eqIdx := strings.IndexAny(s, "= \t\n\r/>")
		if eqIdx < 0 {
			// Remaining is a boolean attribute.
			key := strings.ToLower(s)
			raw = append(raw, htmlAttr{key: key, isBoolean: true})
			m[key] = ""
			break
		}
		key := strings.ToLower(strings.TrimSpace(s[:eqIdx]))
		s = s[eqIdx:]

		if s == "" || s[0] != '=' {
			// Boolean attribute.
			if key != "" {
				raw = append(raw, htmlAttr{key: key, isBoolean: true})
				m[key] = ""
			}
			continue
		}
		// Skip '='.
		s = strings.TrimSpace(s[1:])
		var val string
		if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
			quote := s[0]
			end := strings.IndexByte(s[1:], quote)
			if end < 0 {
				val = s[1:]
				s = ""
			} else {
				val = s[1 : end+1]
				s = s[end+2:]
			}
		} else {
			// Unquoted value — ends at whitespace or >.
			end := strings.IndexAny(s, " \t\n\r>")
			if end < 0 {
				val = s
				s = ""
			} else {
				val = s[:end]
				s = s[end:]
			}
		}
		val = gohtml.UnescapeString(val)
		if key != "" {
			raw = append(raw, htmlAttr{key: key, val: val})
			m[key] = val
		}
	}
	return raw, m
}

// ---------------------------------------------------------------------------
// HTMLToTelegramParser — port of HTMLToTelegramParser in html.py
// ---------------------------------------------------------------------------

type buildingEntityState struct {
	entity tg.MessageEntityClass
	depth  int
}

type openTagEntry struct {
	name string
	meta interface{} // nil | *literalTagSentinel | string (URL for <a>)
}

type htmlParser struct {
	text     string
	utf16Pos int // current UTF-16 offset into text

	entities         []tg.MessageEntityClass
	buildingEntities map[string]*buildingEntityState // keyed by tag name
	openTags         []openTagEntry                  // stack (top = last element)
}

func newHTMLParser() *htmlParser {
	return &htmlParser{
		buildingEntities: make(map[string]*buildingEntityState),
	}
}

func (p *htmlParser) appendText(text string) {
	if text == "" {
		return
	}
	// Measure the UTF-16 width of the new text.
	delta := helpers.UTF16Len(text)
	// Grow all currently-building entity lengths.
	for _, state := range p.buildingEntities {
		helpers.SetEntityLength(state.entity,
			helpers.GetEntityLength(state.entity)+delta)
	}
	p.text += text
	p.utf16Pos += delta
}

// parseTGMention parses "tg://user?id=N" returning the user ID, or -1 on failure.
func parseTGMention(url string) int64 {
	const prefix = "tg://user?id="
	if !strings.HasPrefix(url, prefix) {
		return -1
	}
	id, err := strconv.ParseInt(url[len(prefix):], 10, 64)
	if err != nil {
		return -1
	}
	return id
}

func (p *htmlParser) markLiteralStartTag(tag string, raw string) {
	// Replace the last openTags entry's meta with literalTag.
	if len(p.openTags) > 0 {
		p.openTags[len(p.openTags)-1].meta = literalTag
	}
	// Emit the raw tag text (or reconstruct "<tag>").
	emit := raw
	if emit == "" {
		emit = "<" + tag + ">"
	}
	p.appendText(emit)
}

func (p *htmlParser) handleStartTag(tag string, attrs map[string]string, attrRaw []htmlAttr, raw string) {
	p.openTags = append(p.openTags, openTagEntry{name: tag, meta: nil})

	if !supportedTags[tag] {
		p.markLiteralStartTag(tag, raw)
		return
	}

	args := make(map[string]interface{})
	var entityType string // symbolic name for the entity to build

	// Check for boolean "expandable" attribute.
	hasExpandable := false
	for _, a := range attrRaw {
		if a.key == "expandable" {
			hasExpandable = true
			break
		}
	}

	switch tag {
	case "strong", "b":
		entityType = "bold"
	case "em", "i":
		entityType = "italic"
	case "u":
		entityType = "underline"
	case "del", "s":
		entityType = "strike"
	case "tg-spoiler":
		entityType = "spoiler"
	case "blockquote":
		entityType = "blockquote"
		if hasExpandable {
			val := attrs["expandable"]
			norm := strings.TrimSpace(strings.ToLower(val))
			if norm == "" || norm == "true" || norm == "1" || norm == "yes" || norm == "on" {
				args["collapsed"] = true
			} else {
				args["collapsed"] = nil
			}
		} else {
			args["collapsed"] = nil
		}
	case "pre":
		entityType = "pre"
		args["language"] = ""
	case "code", "mono":
		// Inside <pre>: set the language, do not create a separate entity.
		if preState, ok := p.buildingEntities["pre"]; ok {
			if pre, ok2 := preState.entity.(*tg.MessageEntityPre); ok2 {
				cls := attrs["class"]
				if strings.HasPrefix(cls, "language-") {
					pre.Language = cls[9:]
				}
			}
			// Suppress entity creation.
			entityType = ""
		} else {
			entityType = "code"
		}
	case "a":
		href := attrs["href"]
		if href == "" {
			p.markLiteralStartTag(tag, raw)
			return
		}
		if strings.HasPrefix(href, "mailto:") {
			// MessageEntityEmail — URL is the email address, not stored in entity.
			entityType = "email"
			// Store the URL in the tag meta for use at close time.
			p.openTags[len(p.openTags)-1].meta = href[len("mailto:"):]
		} else if uid := parseTGMention(href); uid >= 0 {
			entityType = "mentionname"
			args["user_id"] = uid
			p.openTags[len(p.openTags)-1].meta = "" // no URL in meta
		} else {
			entityType = "texturl"
			args["url"] = href
			// Store the raw URL in meta to compare with text at close time.
			p.openTags[len(p.openTags)-1].meta = href
		}
	case "tg-emoji":
		emojiID := attrs["emoji-id"]
		if emojiID == "" {
			p.markLiteralStartTag(tag, raw)
			return
		}
		docID, err := strconv.ParseInt(emojiID, 10, 64)
		if err != nil {
			p.markLiteralStartTag(tag, raw)
			return
		}
		entityType = "customemoji"
		args["document_id"] = docID
	case "emoji":
		docIDStr := attrs["document_id"]
		if docIDStr == "" {
			p.markLiteralStartTag(tag, raw)
			return
		}
		docID, err := strconv.ParseInt(docIDStr, 10, 64)
		if err != nil {
			p.markLiteralStartTag(tag, raw)
			return
		}
		entityType = "customemoji"
		args["document_id"] = docID
	}

	if entityType == "" {
		return
	}

	// Increment depth if already building, or create new state.
	if state, ok := p.buildingEntities[tag]; ok {
		state.depth++
		return
	}

	var ent tg.MessageEntityClass
	offset := p.utf16Pos

	switch entityType {
	case "bold":
		ent = &tg.MessageEntityBold{Offset: offset, Length: 0}
	case "italic":
		ent = &tg.MessageEntityItalic{Offset: offset, Length: 0}
	case "underline":
		ent = &tg.MessageEntityUnderline{Offset: offset, Length: 0}
	case "strike":
		ent = &tg.MessageEntityStrike{Offset: offset, Length: 0}
	case "spoiler":
		ent = &tg.MessageEntitySpoiler{Offset: offset, Length: 0}
	case "blockquote":
		ent = &tg.MessageEntityBlockquote{Offset: offset, Length: 0}
	case "code":
		ent = &tg.MessageEntityCode{Offset: offset, Length: 0}
	case "pre":
		lang := ""
		if l, ok := args["language"]; ok {
			lang = l.(string)
		}
		ent = &tg.MessageEntityPre{Offset: offset, Length: 0, Language: lang}
	case "email":
		ent = &tg.MessageEntityEmail{Offset: offset, Length: 0}
	case "texturl":
		url := ""
		if u, ok := args["url"]; ok {
			url = u.(string)
		}
		ent = &tg.MessageEntityTextURL{Offset: offset, Length: 0, URL: url}
	case "mentionname":
		uid := int64(0)
		if u, ok := args["user_id"]; ok {
			uid = u.(int64)
		}
		ent = &tg.MessageEntityMentionName{Offset: offset, Length: 0, UserID: uid}
	case "customemoji":
		docID := int64(0)
		if d, ok := args["document_id"]; ok {
			docID = d.(int64)
		}
		ent = &tg.MessageEntityCustomEmoji{Offset: offset, Length: 0, DocumentID: docID}
	}

	if ent != nil {
		p.buildingEntities[tag] = &buildingEntityState{entity: ent, depth: 1}
	}
}

func (p *htmlParser) handleData(text string) {
	p.appendText(text)
}

func (p *htmlParser) handleEndTag(tag string) {
	keepLiteral := false
	matchedOpen := false

	n := len(p.openTags)
	if n > 0 && p.openTags[n-1].name == tag {
		// Matches the most-recently opened tag.
		matchedOpen = true
		keepLiteral = p.openTags[n-1].meta == literalTag
		p.openTags = p.openTags[:n-1]
	} else {
		// Search further back in the stack.
		for i := n - 2; i >= 0; i-- {
			if p.openTags[i].name == tag {
				matchedOpen = true
				keepLiteral = p.openTags[i].meta == literalTag
				// Remove this entry.
				p.openTags = append(p.openTags[:i], p.openTags[i+1:]...)
				break
			}
		}
	}

	if keepLiteral {
		p.appendText("</" + tag + ">")
	} else if !matchedOpen && !supportedTags[tag] {
		// Unmatched closing tag for unknown element — preserve literally.
		p.appendText("</" + tag + ">")
	}

	state, ok := p.buildingEntities[tag]
	if !ok {
		return
	}

	state.depth--
	if state.depth > 0 {
		return
	}

	delete(p.buildingEntities, tag)
	ent := state.entity

	// For TextUrl: if the entity text equals the URL, downgrade to plain URL.
	if tu, ok := ent.(*tg.MessageEntityTextURL); ok {
		off := tu.Offset
		end := off + tu.Length
		// Extract the entity text from p.text (proper UTF-8).
		entityText := utf16Substring(p.text, off, end)
		if entityText == tu.URL {
			ent = &tg.MessageEntityURL{Offset: tu.Offset, Length: tu.Length}
		}
	}

	p.entities = append(p.entities, ent)
}

// utf16Substring extracts the substring of s corresponding to UTF-16 code
// units [start, end).
func utf16Substring(s string, start, end int) string {
	pos := 0
	startByte := -1
	endByte := len(s)

	for i, r := range s {
		if pos == start {
			startByte = i
		}
		if pos == end {
			endByte = i
			break
		}
		if r >= 0x10000 {
			pos += 2
		} else {
			pos++
		}
	}
	if startByte < 0 {
		return ""
	}
	if startByte > endByte {
		return ""
	}
	return s[startByte:endByte]
}

// ---------------------------------------------------------------------------
// ParseHTML — port of telethon/extensions/html.py parse()
// ---------------------------------------------------------------------------

// ParseHTML parses an HTML-formatted Telegram message and returns the plain
// text and the list of MessageEntityClass values.  It is an exact port of
// Telethon's telethon/extensions/html.py parse().
func ParseHTML(htmlText string) (string, []tg.MessageEntityClass, error) {
	if htmlText == "" {
		return htmlText, nil, nil
	}

	// Pre-process: escape invalid tag syntax (e.g. "<3").
	htmlText = escapeInvalidTagSyntax(htmlText)

	p := newHTMLParser()

	for _, tok := range tokenizeHTML(htmlText) {
		switch tok.typ {
		case tokText:
			p.handleData(tok.data)
		case tokStart, tokSelfClose:
			p.handleStartTag(tok.tag, tok.attrs, tok.attrRaw, tok.raw)
		case tokEnd:
			p.handleEndTag(tok.tag)
		}
	}

	// Strip whitespace and adjust entity offsets.
	text, entities := helpers.StripText(p.text, p.entities)

	// Sort entities by offset (ascending), matching Telethon's final sort.
	sort.Slice(entities, func(i, j int) bool {
		return helpers.GetEntityOffset(entities[i]) < helpers.GetEntityOffset(entities[j])
	})

	return text, entities, nil
}

// ---------------------------------------------------------------------------
// UnparseHTML — port of telethon/extensions/html.py unparse()
// ---------------------------------------------------------------------------

// htmlEscape HTML-escapes plain text for inclusion in HTML output.
// This mirrors Python's html.escape(s, quote=False).
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

// htmlEscapeQ HTML-escapes including double-quotes (for attribute values).
func htmlEscapeQ(s string) string {
	s = htmlEscape(s)
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}

type htmlInsert struct {
	pos     int    // UTF-16 position
	order   int    // secondary sort key (entity index, negative for close)
	content string // HTML fragment to insert
}

// UnparseHTML converts plain text and a list of Telegram message entities back
// to an HTML string.  It is an exact port of Telethon's
// telethon/extensions/html.py unparse().
func UnparseHTML(text string, entities []tg.MessageEntityClass) string {
	if text == "" {
		return text
	}
	if len(entities) == 0 {
		return htmlEscape(text)
	}

	u16 := utf16.Encode([]rune(text))
	n := len(u16)

	var inserts []htmlInsert
	for i, ent := range entities {
		off := helpers.GetEntityOffset(ent)
		end := off + helpers.GetEntityLength(ent)
		entityText := ""
		if off >= 0 && end <= n && off <= end {
			entityText = string(utf16.Decode(u16[off:end]))
		}
		open, close, ok := entityToHTMLTags(ent, entityText)
		if !ok {
			continue
		}
		inserts = append(inserts, htmlInsert{off, i, open})
		inserts = append(inserts, htmlInsert{end, -i, close})
	}

	// Sort ascending by (pos, order).
	sort.Slice(inserts, func(a, b int) bool {
		if inserts[a].pos != inserts[b].pos {
			return inserts[a].pos < inserts[b].pos
		}
		return inserts[a].order < inserts[b].order
	})

	var sb strings.Builder
	prevPos := 0

	for _, ins := range inserts {
		pos := ins.pos
		// Nudge forward out of surrogate pair.
		for helpers.WithinSurrogate(u16, pos) {
			pos++
		}
		if pos > n {
			pos = n
		}
		if pos < prevPos {
			pos = prevPos
		}
		// Escape and emit the plain text up to this position.
		if pos > prevPos {
			seg := string(utf16.Decode(u16[prevPos:pos]))
			sb.WriteString(htmlEscape(seg))
			prevPos = pos
		}
		sb.WriteString(ins.content)
	}

	// Emit remaining text.
	if prevPos < n {
		seg := string(utf16.Decode(u16[prevPos:]))
		sb.WriteString(htmlEscape(seg))
	}

	return sb.String()
}

// entityToHTMLTags returns the opening and closing HTML strings for an entity.
// entityText is the plain-text content covered by the entity (needed for URL
// and email entities).
func entityToHTMLTags(ent tg.MessageEntityClass, entityText string) (open, close string, ok bool) {
	switch e := ent.(type) {
	case *tg.MessageEntityBold:
		return "<strong>", "</strong>", true
	case *tg.MessageEntityItalic:
		return "<em>", "</em>", true
	case *tg.MessageEntityCode:
		return "<code>", "</code>", true
	case *tg.MessageEntityUnderline:
		return "<u>", "</u>", true
	case *tg.MessageEntityStrike:
		return "<del>", "</del>", true
	case *tg.MessageEntitySpoiler:
		return "<tg-spoiler>", "</tg-spoiler>", true
	case *tg.MessageEntityBlockquote:
		// Note: gotd/td v0.89.0 MessageEntityBlockquote has no Collapsed field.
		// We always emit a plain <blockquote>.
		_ = e
		return "<blockquote>", "</blockquote>", true
	case *tg.MessageEntityPre:
		if e.Language != "" {
			lang := htmlEscapeQ(e.Language)
			return fmt.Sprintf("<pre><code class='language-%s'>", lang), "</code></pre>", true
		}
		return "<pre><code>", "</code></pre>", true
	case *tg.MessageEntityEmail:
		return fmt.Sprintf(`<a href="mailto:%s">`, htmlEscapeQ(entityText)), "</a>", true
	case *tg.MessageEntityURL:
		return fmt.Sprintf(`<a href="%s">`, htmlEscapeQ(entityText)), "</a>", true
	case *tg.MessageEntityTextURL:
		return fmt.Sprintf(`<a href="%s">`, htmlEscapeQ(e.URL)), "</a>", true
	case *tg.MessageEntityMentionName:
		return fmt.Sprintf(`<a href="tg://user?id=%d">`, e.UserID), "</a>", true
	case *tg.MessageEntityCustomEmoji:
		return fmt.Sprintf(`<tg-emoji emoji-id="%d">`, e.DocumentID), "</tg-emoji>", true
	}
	return "", "", false
}

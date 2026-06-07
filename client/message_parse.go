package client

import (
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
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
func ParseText(text, parseMode string) (string, []tg.MessageEntityClass, error) {
	switch SanitizeParseMode(parseMode) {
	case ParseModeHTML:
		return HTMLToEntities(text)
	case ParseModeMarkdown:
		return MarkdownToEntities(text)
	default:
		return text, nil, nil
	}
}

// UnparseText converts a plain message and its entity list back to a
// formatted string for the given parse mode.
func UnparseText(text string, entities []tg.MessageEntityClass, parseMode string) string {
	switch SanitizeParseMode(parseMode) {
	case ParseModeHTML:
		return EntitiesToHTML(text, entities)
	case ParseModeMarkdown:
		return EntitiesToMarkdown(text, entities)
	default:
		return text
	}
}

// HTMLToEntities converts HTML-formatted text to plain text + entity list.
// Supported tags: <b>, <strong>, <i>, <em>, <u>, <ins>, <s>, <strike>,
// <del>, <code>, <pre>, <a href="...">.
func HTMLToEntities(html string) (string, []tg.MessageEntityClass, error) {
	var (
		out      strings.Builder
		entities []tg.MessageEntityClass
		stack    []htmlTag // open-tag stack for nesting
	)

	i := 0
	runes := []rune(html)
	n := len(runes)

	for i < n {
		ch := runes[i]
		if ch != '<' {
			out.WriteRune(ch)
			i++
			continue
		}

		// Find the closing '>'
		end := i + 1
		for end < n && runes[end] != '>' {
			end++
		}
		if end >= n {
			// Unclosed tag — treat literally.
			out.WriteRune(ch)
			i++
			continue
		}

		tagContent := string(runes[i+1 : end])
		i = end + 1

		if strings.HasPrefix(tagContent, "/") {
			// Closing tag.
			name := strings.ToLower(strings.TrimSpace(tagContent[1:]))
			// Pop matching open tag.
			for j := len(stack) - 1; j >= 0; j-- {
				if stack[j].name == name || tagAlias(stack[j].name) == name {
					ent := buildHTMLEntity(stack[j], out.Len())
					if ent != nil {
						entities = append(entities, ent)
					}
					stack = append(stack[:j], stack[j+1:]...)
					break
				}
			}
			continue
		}

		// Opening tag (possibly self-closing — ignore for these tags).
		parts := strings.Fields(tagContent)
		if len(parts) == 0 {
			continue
		}
		tagName := strings.ToLower(strings.TrimSuffix(parts[0], "/"))
		attrs := parseHTMLAttrs(tagContent[len(parts[0]):])

		switch tagName {
		case "b", "strong", "i", "em", "u", "ins", "s", "strike", "del", "code", "pre",
			"a", "spoiler", "tg-spoiler":
			stack = append(stack, htmlTag{
				name:   tagName,
				start:  out.Len(),
				attrs:  attrs,
			})
		case "br":
			out.WriteByte('\n')
		}
	}

	return out.String(), entities, nil
}

// EntitiesToHTML converts plain text + entity list to HTML-formatted text.
func EntitiesToHTML(text string, entities []tg.MessageEntityClass) string {
	if len(entities) == 0 {
		return escapeHTML(text)
	}

	runes := []rune(text)
	type event struct {
		pos  int
		open bool
		tag  string
		prio int
	}
	var events []event
	for _, ent := range entities {
		open, close, ok := entityToHTMLTags(ent)
		if !ok {
			continue
		}
		var off, length int
		switch e := ent.(type) {
		case *tg.MessageEntityBold:
			off, length = e.Offset, e.Length
		case *tg.MessageEntityItalic:
			off, length = e.Offset, e.Length
		case *tg.MessageEntityUnderline:
			off, length = e.Offset, e.Length
		case *tg.MessageEntityStrike:
			off, length = e.Offset, e.Length
		case *tg.MessageEntityCode:
			off, length = e.Offset, e.Length
		case *tg.MessageEntityPre:
			off, length = e.Offset, e.Length
		case *tg.MessageEntityTextURL:
			off, length = e.Offset, e.Length
		case *tg.MessageEntitySpoiler:
			off, length = e.Offset, e.Length
		default:
			continue
		}
		events = append(events,
			event{pos: off, open: true, tag: open},
			event{pos: off + length, open: false, tag: close},
		)
	}

	var sb strings.Builder
	prev := 0
	// Simple O(n*m) approach — sufficient for typical message lengths.
	for pos := range runes {
		for _, ev := range events {
			if ev.pos == pos && ev.open {
				sb.WriteString(escapeHTML(string(runes[prev:pos])))
				prev = pos
				sb.WriteString(ev.tag)
			}
		}
		for _, ev := range events {
			if ev.pos == pos && !ev.open {
				sb.WriteString(escapeHTML(string(runes[prev:pos])))
				prev = pos
				sb.WriteString(ev.tag)
			}
		}
	}
	// Flush remaining events at end.
	end := len(runes)
	for _, ev := range events {
		if ev.pos == end && ev.open {
			sb.WriteString(escapeHTML(string(runes[prev:end])))
			prev = end
			sb.WriteString(ev.tag)
		}
	}
	for _, ev := range events {
		if ev.pos == end && !ev.open {
			sb.WriteString(escapeHTML(string(runes[prev:end])))
			prev = end
			sb.WriteString(ev.tag)
		}
	}
	sb.WriteString(escapeHTML(string(runes[prev:])))
	return sb.String()
}

// MarkdownToEntities parses Telegram-style Markdown to plain text + entity list.
// Supported: **bold**, __italic__, `code`, ```pre```, [text](url), ||spoiler||.
func MarkdownToEntities(md string) (string, []tg.MessageEntityClass, error) {
	var (
		out      strings.Builder
		entities []tg.MessageEntityClass
	)

	runes := []rune(md)
	n := len(runes)
	i := 0

	for i < n {
		// Bold: **text**
		if i+1 < n && runes[i] == '*' && runes[i+1] == '*' {
			end := findClosing(runes, i+2, "**")
			if end >= 0 {
				start := out.Len()
				inner, innerEnts, _ := MarkdownToEntities(string(runes[i+2 : end]))
				out.WriteString(inner)
				entities = append(entities, &tg.MessageEntityBold{Offset: start, Length: out.Len() - start})
				entities = append(entities, shiftEntities(innerEnts, start)...)
				i = end + 2
				continue
			}
		}
		// Italic: __text__
		if i+1 < n && runes[i] == '_' && runes[i+1] == '_' {
			end := findClosing(runes, i+2, "__")
			if end >= 0 {
				start := out.Len()
				inner, innerEnts, _ := MarkdownToEntities(string(runes[i+2 : end]))
				out.WriteString(inner)
				entities = append(entities, &tg.MessageEntityItalic{Offset: start, Length: out.Len() - start})
				entities = append(entities, shiftEntities(innerEnts, start)...)
				i = end + 2
				continue
			}
		}
		// Spoiler: ||text||
		if i+1 < n && runes[i] == '|' && runes[i+1] == '|' {
			end := findClosing(runes, i+2, "||")
			if end >= 0 {
				start := out.Len()
				inner, innerEnts, _ := MarkdownToEntities(string(runes[i+2 : end]))
				out.WriteString(inner)
				entities = append(entities, &tg.MessageEntitySpoiler{Offset: start, Length: out.Len() - start})
				entities = append(entities, shiftEntities(innerEnts, start)...)
				i = end + 2
				continue
			}
		}
		// Pre: ```text```
		if i+2 < n && runes[i] == '`' && runes[i+1] == '`' && runes[i+2] == '`' {
			end := findClosing(runes, i+3, "```")
			if end >= 0 {
				start := out.Len()
				inner := string(runes[i+3 : end])
				out.WriteString(inner)
				entities = append(entities, &tg.MessageEntityPre{Offset: start, Length: len([]rune(inner))})
				i = end + 3
				continue
			}
		}
		// Code: `text`
		if runes[i] == '`' {
			end := findClosing(runes, i+1, "`")
			if end >= 0 {
				start := out.Len()
				inner := string(runes[i+1 : end])
				out.WriteString(inner)
				entities = append(entities, &tg.MessageEntityCode{Offset: start, Length: len([]rune(inner))})
				i = end + 1
				continue
			}
		}
		// Link: [text](url)
		if runes[i] == '[' {
			closeBracket := findClosing(runes, i+1, "]")
			if closeBracket >= 0 && closeBracket+1 < n && runes[closeBracket+1] == '(' {
				closeParen := findClosing(runes, closeBracket+2, ")")
				if closeParen >= 0 {
					start := out.Len()
					linkText := string(runes[i+1 : closeBracket])
					linkURL := string(runes[closeBracket+2 : closeParen])
					out.WriteString(linkText)
					entities = append(entities, &tg.MessageEntityTextURL{
						Offset: start,
						Length: len([]rune(linkText)),
						URL:    linkURL,
					})
					i = closeParen + 1
					continue
				}
			}
		}
		out.WriteRune(runes[i])
		i++
	}
	return out.String(), entities, nil
}

// EntitiesToMarkdown converts plain text + entity list to Telegram-style Markdown.
func EntitiesToMarkdown(text string, entities []tg.MessageEntityClass) string {
	// For simplicity, delegate to HTML and note that a full round-trip
	// Markdown serialiser would require a more complex approach.
	// We implement a straightforward version here.
	if len(entities) == 0 {
		return text
	}
	runes := []rune(text)

	type insertion struct {
		pos  int
		text string
		end  bool // true = close, false = open
	}
	var ins []insertion

	for _, ent := range entities {
		var off, length int
		var open, close string
		switch e := ent.(type) {
		case *tg.MessageEntityBold:
			off, length, open, close = e.Offset, e.Length, "**", "**"
		case *tg.MessageEntityItalic:
			off, length, open, close = e.Offset, e.Length, "__", "__"
		case *tg.MessageEntityCode:
			off, length, open, close = e.Offset, e.Length, "`", "`"
		case *tg.MessageEntityPre:
			off, length, open, close = e.Offset, e.Length, "```", "```"
		case *tg.MessageEntityTextURL:
			off, length = e.Offset, e.Length
			open = "["
			close = fmt.Sprintf("](%s)", e.URL)
		case *tg.MessageEntitySpoiler:
			off, length, open, close = e.Offset, e.Length, "||", "||"
		default:
			continue
		}
		ins = append(ins, insertion{pos: off, text: open, end: false})
		ins = append(ins, insertion{pos: off + length, text: close, end: true})
	}

	var sb strings.Builder
	for pos, r := range runes {
		for _, in := range ins {
			if in.pos == pos && !in.end {
				sb.WriteString(in.text)
			}
		}
		for _, in := range ins {
			if in.pos == pos && in.end {
				sb.WriteString(in.text)
			}
		}
		sb.WriteRune(r)
	}
	end := len(runes)
	for _, in := range ins {
		if in.pos == end {
			sb.WriteString(in.text)
		}
	}
	return sb.String()
}

// StripText removes all entity formatting markers, returning plain text.
// For text-based entities no modification is needed (the text itself is unaffected).
func StripText(text string, entities []tg.MessageEntityClass) string {
	_ = entities // entities don't alter the underlying rune content
	return text
}

// --------------------------------------------------------------------------
// Internal helpers
// --------------------------------------------------------------------------

type htmlTag struct {
	name  string
	start int
	attrs map[string]string
}

func buildHTMLEntity(tag htmlTag, end int) tg.MessageEntityClass {
	length := end - tag.start
	if length <= 0 {
		return nil
	}
	switch tag.name {
	case "b", "strong":
		return &tg.MessageEntityBold{Offset: tag.start, Length: length}
	case "i", "em":
		return &tg.MessageEntityItalic{Offset: tag.start, Length: length}
	case "u", "ins":
		return &tg.MessageEntityUnderline{Offset: tag.start, Length: length}
	case "s", "strike", "del":
		return &tg.MessageEntityStrike{Offset: tag.start, Length: length}
	case "code":
		return &tg.MessageEntityCode{Offset: tag.start, Length: length}
	case "pre":
		return &tg.MessageEntityPre{Offset: tag.start, Length: length}
	case "a":
		href := tag.attrs["href"]
		if href == "" {
			return nil
		}
		return &tg.MessageEntityTextURL{Offset: tag.start, Length: length, URL: href}
	case "spoiler", "tg-spoiler":
		return &tg.MessageEntitySpoiler{Offset: tag.start, Length: length}
	}
	return nil
}

func tagAlias(name string) string {
	switch name {
	case "b":
		return "strong"
	case "strong":
		return "b"
	case "i":
		return "em"
	case "em":
		return "i"
	case "u":
		return "ins"
	case "ins":
		return "u"
	case "s":
		return "strike"
	case "strike":
		return "del"
	}
	return ""
}

func parseHTMLAttrs(s string) map[string]string {
	attrs := map[string]string{}
	s = strings.TrimSpace(s)
	for s != "" {
		eqIdx := strings.IndexByte(s, '=')
		if eqIdx < 0 {
			break
		}
		key := strings.ToLower(strings.TrimSpace(s[:eqIdx]))
		s = strings.TrimSpace(s[eqIdx+1:])
		var val string
		if len(s) > 0 && (s[0] == '"' || s[0] == '\'') {
			quote := s[0]
			end := strings.IndexByte(s[1:], quote)
			if end < 0 {
				break
			}
			val = s[1 : end+1]
			s = strings.TrimSpace(s[end+2:])
		} else {
			spIdx := strings.IndexAny(s, " \t\r\n")
			if spIdx < 0 {
				val = s
				s = ""
			} else {
				val = s[:spIdx]
				s = strings.TrimSpace(s[spIdx:])
			}
		}
		attrs[key] = val
	}
	return attrs
}

func entityToHTMLTags(ent tg.MessageEntityClass) (open, close string, ok bool) {
	switch ent.(type) {
	case *tg.MessageEntityBold:
		return "<b>", "</b>", true
	case *tg.MessageEntityItalic:
		return "<i>", "</i>", true
	case *tg.MessageEntityUnderline:
		return "<u>", "</u>", true
	case *tg.MessageEntityStrike:
		return "<s>", "</s>", true
	case *tg.MessageEntityCode:
		return "<code>", "</code>", true
	case *tg.MessageEntityPre:
		return "<pre>", "</pre>", true
	case *tg.MessageEntityTextURL:
		e := ent.(*tg.MessageEntityTextURL)
		return fmt.Sprintf(`<a href="%s">`, escapeHTML(e.URL)), "</a>", true
	case *tg.MessageEntitySpoiler:
		return `<tg-spoiler>`, `</tg-spoiler>`, true
	}
	return "", "", false
}

func escapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&#34;")
	return s
}

// findClosing finds the position of the closing marker starting from pos.
// Returns the start index of the marker, or -1 if not found.
func findClosing(runes []rune, pos int, marker string) int {
	m := []rune(marker)
	ml := len(m)
	n := len(runes)
	for i := pos; i <= n-ml; i++ {
		match := true
		for j := 0; j < ml; j++ {
			if runes[i+j] != m[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// shiftEntities returns a copy of entities with offsets shifted by delta.
func shiftEntities(entities []tg.MessageEntityClass, delta int) []tg.MessageEntityClass {
	out := make([]tg.MessageEntityClass, 0, len(entities))
	for _, ent := range entities {
		switch e := ent.(type) {
		case *tg.MessageEntityBold:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntityItalic:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntityCode:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntityPre:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntityTextURL:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntitySpoiler:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntityUnderline:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		case *tg.MessageEntityStrike:
			cp := *e
			cp.Offset += delta
			out = append(out, &cp)
		default:
			out = append(out, ent)
		}
	}
	return out
}

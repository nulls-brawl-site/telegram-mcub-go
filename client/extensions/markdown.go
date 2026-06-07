package extensions

// Markdown parser — exact Go port of telethon/extensions/markdown.py
//
// Supports:
//   **bold**   __italic__   ~~strike~~   `code`   ```pre```
//   [text](url)  →  MessageEntityTextUrl
//   [text](tg://user?id=N)  →  MessageEntityMentionName
//
// Entity offsets and lengths are in UTF-16 code units (Telegram's encoding).
// No nesting is supported inside code or pre blocks.

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/helpers"
)

// defaultDelimiters maps delimiter string → entity type tag.
var defaultDelimiters = []mdDelim{
	{"```", "pre"},
	{"**", "bold"},
	{"__", "italic"},
	{"~~", "strike"},
	{"`", "code"},
}

type mdDelim struct {
	str string
	typ string
}

// defaultURLRe mirrors DEFAULT_URL_RE in markdown.py.
var defaultURLRe = regexp.MustCompile(`\[([^\]]*?)\]\(([\s\S]*?)\)`)

// ---------------------------------------------------------------------------
// ParseMarkdown — port of telethon/extensions/markdown.py parse()
// ---------------------------------------------------------------------------

// ParseMarkdown parses Telegram-style Markdown text and returns the plain
// text and message entities.  It is an exact port of Telethon's
// telethon/extensions/markdown.py parse().
//
// The algorithm operates on a UTF-16 view of the message (by working with
// []uint16) so that all offsets and lengths are correct for Telegram.
func ParseMarkdown(message string) (string, []tg.MessageEntityClass, error) {
	if message == "" {
		return message, nil, nil
	}

	// Work in UTF-16 to get correct offsets for emoji.
	// We represent the "working message" as a []uint16 slice (mirroring Python's
	// surrogate-encoded string), and convert back to string at the end.
	u16 := helpers.ToUTF16(message)

	var result []tg.MessageEntityClass

	// Build a list of delimiters sorted longest-first (so ``` beats `).
	delims := defaultDelimiters // already sorted longest-first

	i := 0
	for i < len(u16) {
		// Try each delimiter at position i.
		matched := false
		for _, d := range delims {
			du16 := helpers.ToUTF16(d.str)
			dl := len(du16)

			if !u16HasPrefix(u16[i:], du16) {
				continue
			}

			// Search for the closing delimiter starting at i+dl+1
			// (+1 to avoid matching right after, e.g. "****").
			end := u16IndexFrom(u16, du16, i+dl+1)
			if end < 0 {
				continue
			}

			// Extract the content between delimiters.
			content := u16[i+dl : end]

			// Remove the delimiters from u16.
			// New u16 = u16[:i] + content + u16[end+dl:]
			var newU16 []uint16
			newU16 = append(newU16, u16[:i]...)
			newU16 = append(newU16, content...)
			newU16 = append(newU16, u16[end+dl:]...)
			u16 = newU16

			// Adjust existing entities for the removed delimiters.
			for _, ent := range result {
				off := helpers.GetEntityOffset(ent)
				length := helpers.GetEntityLength(ent)
				entEnd := off + length
				if entEnd > i {
					if off <= i && entEnd >= end+dl {
						// Fully enclosing: reduce length by 2*dl.
						helpers.SetEntityLength(ent, length-2*dl)
					} else {
						helpers.SetEntityLength(ent, length-dl)
					}
				}
			}

			// Create the new entity.
			var ent tg.MessageEntityClass
			entityLen := end - i - dl // length after delimiter removal
			switch d.typ {
			case "bold":
				ent = &tg.MessageEntityBold{Offset: i, Length: entityLen}
			case "italic":
				ent = &tg.MessageEntityItalic{Offset: i, Length: entityLen}
			case "strike":
				ent = &tg.MessageEntityStrike{Offset: i, Length: entityLen}
			case "code":
				ent = &tg.MessageEntityCode{Offset: i, Length: entityLen}
			case "pre":
				ent = &tg.MessageEntityPre{Offset: i, Length: entityLen, Language: ""}
			}
			result = append(result, ent)

			// No nested entities inside code/pre.
			if d.typ == "code" || d.typ == "pre" {
				i = end - dl
			}

			matched = true
			break
		}

		if matched {
			continue
		}

		// Try URL pattern at position i.
		if defaultURLRe != nil {
			// Convert current u16 back to string for regex matching.
			// We match only at position i.
			currentStr := helpers.FromUTF16(u16)
			// Find the byte offset corresponding to UTF-16 position i.
			byteOff := utf16PosToByteOff(currentStr, i)
			if byteOff >= 0 {
				loc := defaultURLRe.FindStringSubmatchIndex(currentStr[byteOff:])
				if loc != nil && loc[0] == 0 {
					// Adjust indices to be relative to start of currentStr.
					for k := range loc {
						if loc[k] >= 0 {
							loc[k] += byteOff
						}
					}

					fullMatch := currentStr[loc[0]:loc[1]]
					linkText := currentStr[loc[2]:loc[3]]
					linkURL := currentStr[loc[4]:loc[5]]

					// Replace the full match with just the link text.
					newStr := currentStr[:loc[0]] + linkText + currentStr[loc[1]:]
					u16 = helpers.ToUTF16(newStr)

					// Compute how many UTF-16 units were removed.
					delimSize := helpers.UTF16Len(fullMatch) - helpers.UTF16Len(linkText)

					// Adjust existing entities.
					for _, ent := range result {
						if helpers.GetEntityOffset(ent)+helpers.GetEntityLength(ent) > i {
							helpers.SetEntityLength(ent,
								helpers.GetEntityLength(ent)-delimSize)
						}
					}

					// Create the URL entity.
					linkTextU16Len := helpers.UTF16Len(linkText)
					var urlEnt tg.MessageEntityClass
					if uid := parseTGMention(linkURL); uid >= 0 {
						urlEnt = &tg.MessageEntityMentionName{
							Offset: i, Length: linkTextU16Len, UserID: uid,
						}
					} else {
						urlEnt = &tg.MessageEntityTextURL{
							Offset: i, Length: linkTextU16Len, URL: linkURL,
						}
					}
					result = append(result, urlEnt)
					i += linkTextU16Len
					continue
				}
			}
		}

		i++
	}

	// Strip whitespace and adjust entities.
	finalStr, result := helpers.StripText(helpers.FromUTF16(u16), result)
	return finalStr, result, nil
}

// u16HasPrefix returns true if u starts with prefix.
func u16HasPrefix(u, prefix []uint16) bool {
	if len(u) < len(prefix) {
		return false
	}
	for i, v := range prefix {
		if u[i] != v {
			return false
		}
	}
	return true
}

// u16IndexFrom finds the first occurrence of needle in haystack starting at
// position from.  Returns -1 if not found.
func u16IndexFrom(haystack, needle []uint16, from int) int {
	nl := len(needle)
	for i := from; i+nl <= len(haystack); i++ {
		if u16HasPrefix(haystack[i:], needle) {
			return i
		}
	}
	return -1
}

// utf16PosToByteOff converts a UTF-16 position to a byte offset in s.
// Returns -1 if pos is out of range.
func utf16PosToByteOff(s string, pos int) int {
	u16Pos := 0
	for i, r := range s {
		if u16Pos == pos {
			return i
		}
		if r >= 0x10000 {
			u16Pos += 2
		} else {
			u16Pos++
		}
	}
	if u16Pos == pos {
		return len(s)
	}
	return -1
}

// ---------------------------------------------------------------------------
// UnparseMarkdown — port of telethon/extensions/markdown.py unparse()
// ---------------------------------------------------------------------------

// defaultMDDelimiters maps entity type to its Markdown delimiter string.
var mdDelimByType = map[string]string{
	"bold":   "**",
	"italic": "__",
	"strike": "~~",
	"code":   "`",
	"pre":    "```",
}

type mdInsert struct {
	pos     int
	order   int
	content string
}

// UnparseMarkdown converts plain text and message entities back to
// Telegram-style Markdown.  It is a port of Telethon's
// telethon/extensions/markdown.py unparse().
func UnparseMarkdown(text string, entities []tg.MessageEntityClass) string {
	if text == "" || len(entities) == 0 {
		return text
	}

	u16 := utf16.Encode([]rune(text))
	n := len(u16)

	var inserts []mdInsert
	for i, ent := range entities {
		off := helpers.GetEntityOffset(ent)
		end := off + helpers.GetEntityLength(ent)

		switch e := ent.(type) {
		case *tg.MessageEntityBold:
			inserts = append(inserts, mdInsert{off, i, "**"})
			inserts = append(inserts, mdInsert{end, -i, "**"})
		case *tg.MessageEntityItalic:
			inserts = append(inserts, mdInsert{off, i, "__"})
			inserts = append(inserts, mdInsert{end, -i, "__"})
		case *tg.MessageEntityStrike:
			inserts = append(inserts, mdInsert{off, i, "~~"})
			inserts = append(inserts, mdInsert{end, -i, "~~"})
		case *tg.MessageEntityCode:
			inserts = append(inserts, mdInsert{off, i, "`"})
			inserts = append(inserts, mdInsert{end, -i, "`"})
		case *tg.MessageEntityPre:
			inserts = append(inserts, mdInsert{off, i, "```"})
			inserts = append(inserts, mdInsert{end, -i, "```"})
		case *tg.MessageEntityTextURL:
			inserts = append(inserts, mdInsert{off, i, "["})
			inserts = append(inserts, mdInsert{end, -i, "](" + e.URL + ")"})
		case *tg.MessageEntityMentionName:
			inserts = append(inserts, mdInsert{off, i, "["})
			inserts = append(inserts, mdInsert{end, -i, formatMentionClose(e.UserID)})
		}
	}

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
		for helpers.WithinSurrogate(u16, pos) {
			pos++
		}
		if pos > n {
			pos = n
		}
		if pos < prevPos {
			pos = prevPos
		}
		if pos > prevPos {
			seg := string(utf16.Decode(u16[prevPos:pos]))
			sb.WriteString(seg)
			prevPos = pos
		}
		sb.WriteString(ins.content)
	}

	if prevPos < n {
		sb.WriteString(string(utf16.Decode(u16[prevPos:])))
	}

	return sb.String()
}

// formatMentionClose formats the closing part of a MentionName markdown link.
func formatMentionClose(userID int64) string {
	return "](tg://user?id=" + int64ToString(userID) + ")"
}

// int64ToString converts an int64 to its decimal string representation.
func int64ToString(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

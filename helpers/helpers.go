// Package helpers provides UTF-16 surrogate-pair utilities needed by the HTML
// and Markdown parsers.  This is a Go port of telethon/helpers.py focusing on
// the functions used by the extension parsers: add_surrogate, del_surrogate,
// within_surrogate, and strip_text.
//
// Telegram entity offsets and lengths are measured in UTF-16 code units.
// Characters in the Supplementary Multilingual Plane (U+10000–U+10FFFF) —
// such as most emoji — occupy TWO code units (a surrogate pair) in UTF-16,
// but only ONE code point in Python/Go strings.  Telethon handles this by
// converting the string to a "surrogate-encoded" form before counting.
//
// In Go we avoid literal surrogate code-points in strings (they are not valid
// UTF-8) and instead work with []uint16 slices or compute the UTF-16 width
// on the fly.
package helpers

import (
	"strings"
	"unicode/utf16"

	"github.com/gotd/td/tg"
)

// UTF16Len returns the number of UTF-16 code units required to encode s.
// BMP characters cost 1 unit; SMP characters (≥ U+10000) cost 2.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// UTF16RuneLen returns the number of UTF-16 code units for a single rune.
func UTF16RuneLen(r rune) int {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// ToUTF16 converts a Go string to a slice of UTF-16 code units.
func ToUTF16(s string) []uint16 {
	return utf16.Encode([]rune(s))
}

// FromUTF16 converts a slice of UTF-16 code units to a Go string.
func FromUTF16(u []uint16) string {
	return string(utf16.Decode(u))
}

// WithinSurrogate reports whether UTF-16 offset i falls between a high
// surrogate (U+D800–U+DBFF) at i-1 and a low surrogate (U+DC00–U+DFFF)
// at i — i.e. it is in the middle of a surrogate pair and would split an
// emoji.
func WithinSurrogate(u []uint16, i int) bool {
	if i <= 0 || i >= len(u) {
		return false
	}
	return u[i-1] >= 0xD800 && u[i-1] <= 0xDBFF &&
		u[i] >= 0xDC00 && u[i] <= 0xDFFF
}

// ---------------------------------------------------------------------------
// Entity offset/length mutators via type-switch (gotd/td has no setter interface)
// ---------------------------------------------------------------------------

// GetEntityOffset returns the UTF-16 offset stored in any MessageEntityClass.
func GetEntityOffset(e tg.MessageEntityClass) int {
	switch v := e.(type) {
	case *tg.MessageEntityUnknown:
		return v.Offset
	case *tg.MessageEntityMention:
		return v.Offset
	case *tg.MessageEntityHashtag:
		return v.Offset
	case *tg.MessageEntityBotCommand:
		return v.Offset
	case *tg.MessageEntityURL:
		return v.Offset
	case *tg.MessageEntityEmail:
		return v.Offset
	case *tg.MessageEntityBold:
		return v.Offset
	case *tg.MessageEntityItalic:
		return v.Offset
	case *tg.MessageEntityCode:
		return v.Offset
	case *tg.MessageEntityPre:
		return v.Offset
	case *tg.MessageEntityTextURL:
		return v.Offset
	case *tg.MessageEntityMentionName:
		return v.Offset
	case *tg.InputMessageEntityMentionName:
		return v.Offset
	case *tg.MessageEntityPhone:
		return v.Offset
	case *tg.MessageEntityCashtag:
		return v.Offset
	case *tg.MessageEntityUnderline:
		return v.Offset
	case *tg.MessageEntityStrike:
		return v.Offset
	case *tg.MessageEntityBankCard:
		return v.Offset
	case *tg.MessageEntitySpoiler:
		return v.Offset
	case *tg.MessageEntityCustomEmoji:
		return v.Offset
	case *tg.MessageEntityBlockquote:
		return v.Offset
	}
	return 0
}

// GetEntityLength returns the UTF-16 length stored in any MessageEntityClass.
func GetEntityLength(e tg.MessageEntityClass) int {
	switch v := e.(type) {
	case *tg.MessageEntityUnknown:
		return v.Length
	case *tg.MessageEntityMention:
		return v.Length
	case *tg.MessageEntityHashtag:
		return v.Length
	case *tg.MessageEntityBotCommand:
		return v.Length
	case *tg.MessageEntityURL:
		return v.Length
	case *tg.MessageEntityEmail:
		return v.Length
	case *tg.MessageEntityBold:
		return v.Length
	case *tg.MessageEntityItalic:
		return v.Length
	case *tg.MessageEntityCode:
		return v.Length
	case *tg.MessageEntityPre:
		return v.Length
	case *tg.MessageEntityTextURL:
		return v.Length
	case *tg.MessageEntityMentionName:
		return v.Length
	case *tg.InputMessageEntityMentionName:
		return v.Length
	case *tg.MessageEntityPhone:
		return v.Length
	case *tg.MessageEntityCashtag:
		return v.Length
	case *tg.MessageEntityUnderline:
		return v.Length
	case *tg.MessageEntityStrike:
		return v.Length
	case *tg.MessageEntityBankCard:
		return v.Length
	case *tg.MessageEntitySpoiler:
		return v.Length
	case *tg.MessageEntityCustomEmoji:
		return v.Length
	case *tg.MessageEntityBlockquote:
		return v.Length
	}
	return 0
}

// SetEntityOffset sets the UTF-16 offset on any MessageEntityClass.
func SetEntityOffset(e tg.MessageEntityClass, offset int) {
	switch v := e.(type) {
	case *tg.MessageEntityUnknown:
		v.Offset = offset
	case *tg.MessageEntityMention:
		v.Offset = offset
	case *tg.MessageEntityHashtag:
		v.Offset = offset
	case *tg.MessageEntityBotCommand:
		v.Offset = offset
	case *tg.MessageEntityURL:
		v.Offset = offset
	case *tg.MessageEntityEmail:
		v.Offset = offset
	case *tg.MessageEntityBold:
		v.Offset = offset
	case *tg.MessageEntityItalic:
		v.Offset = offset
	case *tg.MessageEntityCode:
		v.Offset = offset
	case *tg.MessageEntityPre:
		v.Offset = offset
	case *tg.MessageEntityTextURL:
		v.Offset = offset
	case *tg.MessageEntityMentionName:
		v.Offset = offset
	case *tg.InputMessageEntityMentionName:
		v.Offset = offset
	case *tg.MessageEntityPhone:
		v.Offset = offset
	case *tg.MessageEntityCashtag:
		v.Offset = offset
	case *tg.MessageEntityUnderline:
		v.Offset = offset
	case *tg.MessageEntityStrike:
		v.Offset = offset
	case *tg.MessageEntityBankCard:
		v.Offset = offset
	case *tg.MessageEntitySpoiler:
		v.Offset = offset
	case *tg.MessageEntityCustomEmoji:
		v.Offset = offset
	case *tg.MessageEntityBlockquote:
		v.Offset = offset
	}
}

// SetEntityLength sets the UTF-16 length on any MessageEntityClass.
func SetEntityLength(e tg.MessageEntityClass, length int) {
	switch v := e.(type) {
	case *tg.MessageEntityUnknown:
		v.Length = length
	case *tg.MessageEntityMention:
		v.Length = length
	case *tg.MessageEntityHashtag:
		v.Length = length
	case *tg.MessageEntityBotCommand:
		v.Length = length
	case *tg.MessageEntityURL:
		v.Length = length
	case *tg.MessageEntityEmail:
		v.Length = length
	case *tg.MessageEntityBold:
		v.Length = length
	case *tg.MessageEntityItalic:
		v.Length = length
	case *tg.MessageEntityCode:
		v.Length = length
	case *tg.MessageEntityPre:
		v.Length = length
	case *tg.MessageEntityTextURL:
		v.Length = length
	case *tg.MessageEntityMentionName:
		v.Length = length
	case *tg.InputMessageEntityMentionName:
		v.Length = length
	case *tg.MessageEntityPhone:
		v.Length = length
	case *tg.MessageEntityCashtag:
		v.Length = length
	case *tg.MessageEntityUnderline:
		v.Length = length
	case *tg.MessageEntityStrike:
		v.Length = length
	case *tg.MessageEntityBankCard:
		v.Length = length
	case *tg.MessageEntitySpoiler:
		v.Length = length
	case *tg.MessageEntityCustomEmoji:
		v.Length = length
	case *tg.MessageEntityBlockquote:
		v.Length = length
	}
}

// ---------------------------------------------------------------------------
// StripText — port of telethon/helpers.py strip_text()
// ---------------------------------------------------------------------------

// StripText strips leading and trailing whitespace from text (which is a
// proper UTF-8 string, NOT surrogate-encoded) and adjusts the entity
// offsets/lengths accordingly, removing zero-length or out-of-bounds
// entities.  Returns the stripped text.
//
// Entity offsets are in UTF-16 code units.  Because all Unicode whitespace
// characters are in the BMP (≤ U+FFFF) they each occupy exactly one UTF-16
// code unit, so rune count == UTF-16 unit count for whitespace.
func StripText(text string, entities []tg.MessageEntityClass) (string, []tg.MessageEntityClass) {
	if len(entities) == 0 {
		return strings.TrimSpace(text), entities
	}

	// Count UTF-16 units in the leading whitespace.
	leftOffset := 0
	for _, r := range text {
		if r == ' ' || r == '\n' || r == '\t' || r == '\r' || r == '\f' || r == '\v' || r == '\u00a0' || r == '\u200b' {
			leftOffset += UTF16RuneLen(r)
		} else {
			break
		}
	}

	stripped := strings.TrimSpace(text)
	lenFinal := UTF16Len(stripped)

	out := entities[:0]
	for _, e := range entities {
		off := GetEntityOffset(e)
		length := GetEntityLength(e)

		if length == 0 {
			continue
		}

		end := off + length

		if end <= leftOffset {
			// Entirely in the stripped leading whitespace — drop.
			continue
		}

		if off >= leftOffset {
			// Entirely after the stripped leading whitespace.
			off -= leftOffset
		} else {
			// Spans the left boundary.
			length = end - leftOffset
			off = 0
		}

		// Now adjust for trailing whitespace.
		end = off + length
		if off >= lenFinal {
			// Entirely in the stripped trailing whitespace — drop.
			continue
		}
		if end > lenFinal {
			length = lenFinal - off
		}
		if length == 0 {
			continue
		}

		SetEntityOffset(e, off)
		SetEntityLength(e, length)
		out = append(out, e)
	}

	return stripped, out
}

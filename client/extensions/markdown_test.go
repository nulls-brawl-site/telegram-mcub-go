package extensions_test

import (
	"testing"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/client/extensions"
)

// ─────────────────────────────────────────────────────────────────────────────
// ParseMarkdown
// ─────────────────────────────────────────────────────────────────────────────

func TestParseMarkdownBold(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("**hello**")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Errorf("text: got %q, want %q", text, "hello")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityBold", entities[0])
	}
	if bold.Offset != 0 || bold.Length != 5 {
		t.Errorf("offset/length: got %d/%d, want 0/5", bold.Offset, bold.Length)
	}
}

func TestParseMarkdownItalic(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("__italic__")
	if err != nil {
		t.Fatal(err)
	}
	if text != "italic" {
		t.Errorf("text: got %q, want %q", text, "italic")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	it, ok := entities[0].(*tg.MessageEntityItalic)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityItalic", entities[0])
	}
	if it.Offset != 0 || it.Length != 6 {
		t.Errorf("offset/length: got %d/%d, want 0/6", it.Offset, it.Length)
	}
}

func TestParseMarkdownCode(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("`code`")
	if err != nil {
		t.Fatal(err)
	}
	if text != "code" {
		t.Errorf("text: got %q, want %q", text, "code")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	code, ok := entities[0].(*tg.MessageEntityCode)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityCode", entities[0])
	}
	if code.Offset != 0 || code.Length != 4 {
		t.Errorf("offset/length: got %d/%d, want 0/4", code.Offset, code.Length)
	}
}

func TestParseMarkdownPre(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("```\ncode\n```")
	if err != nil {
		t.Fatal(err)
	}
	// The delimiter content is "\ncode\n"; StripText strips leading/trailing whitespace.
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	pre, ok := entities[0].(*tg.MessageEntityPre)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityPre", entities[0])
	}
	_ = pre
	if text == "" {
		t.Error("text should not be empty")
	}
}

func TestParseMarkdownPreSimple(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("```abc```")
	if err != nil {
		t.Fatal(err)
	}
	if text != "abc" {
		t.Errorf("text: got %q, want %q", text, "abc")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityPre); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityPre", entities[0])
	}
}

func TestParseMarkdownStrike(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("~~strike~~")
	if err != nil {
		t.Fatal(err)
	}
	if text != "strike" {
		t.Errorf("text: got %q, want %q", text, "strike")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityStrike); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityStrike", entities[0])
	}
}

func TestParseMarkdownURL(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("[text](https://t.me)")
	if err != nil {
		t.Fatal(err)
	}
	if text != "text" {
		t.Errorf("text: got %q, want %q", text, "text")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	tu, ok := entities[0].(*tg.MessageEntityTextURL)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityTextURL", entities[0])
	}
	if tu.URL != "https://t.me" {
		t.Errorf("URL: got %q, want %q", tu.URL, "https://t.me")
	}
	if tu.Offset != 0 || tu.Length != 4 {
		t.Errorf("offset/length: got %d/%d, want 0/4", tu.Offset, tu.Length)
	}
}

func TestParseMarkdownMention(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("[name](tg://user?id=123)")
	if err != nil {
		t.Fatal(err)
	}
	if text != "name" {
		t.Errorf("text: got %q, want %q", text, "name")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	mn, ok := entities[0].(*tg.MessageEntityMentionName)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityMentionName", entities[0])
	}
	if mn.UserID != 123 {
		t.Errorf("UserID: got %d, want 123", mn.UserID)
	}
}

// Inside backtick code, markdown delimiters must not be processed.
func TestParseMarkdownNoNestingInCode(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("`**bold inside code**`")
	if err != nil {
		t.Fatal(err)
	}
	if text != "**bold inside code**" {
		t.Errorf("text: got %q, want %q", text, "**bold inside code**")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1 (only code, no bold)", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityCode); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityCode", entities[0])
	}
}

func TestParseMarkdownEmpty(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("")
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Errorf("text: got %q, want empty", text)
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0", len(entities))
	}
}

func TestParseMarkdownPlainText(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hello world" {
		t.Errorf("text: got %q, want %q", text, "hello world")
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0", len(entities))
	}
}

func TestParseMarkdownMultiple(t *testing.T) {
	text, entities, err := extensions.ParseMarkdown("**bold** __italic__ ~~strike~~")
	if err != nil {
		t.Fatal(err)
	}
	if text != "bold italic strike" {
		t.Errorf("text: got %q, want %q", text, "bold italic strike")
	}
	if len(entities) != 3 {
		t.Fatalf("entity count: got %d, want 3", len(entities))
	}
}

func TestParseMarkdownEmojiOffset(t *testing.T) {
	// "🔮 " is 2 UTF-16 units + 1 space = 3 units before "bold".
	text, entities, err := extensions.ParseMarkdown("🔮 **bold**")
	if err != nil {
		t.Fatal(err)
	}
	if text != "🔮 bold" {
		t.Errorf("text: got %q, want %q", text, "🔮 bold")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityBold", entities[0])
	}
	// 🔮(0,1) sp(2) → bold starts at UTF-16 offset 3
	if bold.Offset != 3 {
		t.Errorf("bold offset: got %d, want 3 (emoji counted as 2 UTF-16 units)", bold.Offset)
	}
	if bold.Length != 4 {
		t.Errorf("bold length: got %d, want 4", bold.Length)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UnparseMarkdown
// ─────────────────────────────────────────────────────────────────────────────

func TestUnparseMarkdownBold(t *testing.T) {
	result := extensions.UnparseMarkdown("hello", []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: 5},
	})
	if result != "**hello**" {
		t.Errorf("got %q, want %q", result, "**hello**")
	}
}

func TestUnparseMarkdownItalic(t *testing.T) {
	result := extensions.UnparseMarkdown("world", []tg.MessageEntityClass{
		&tg.MessageEntityItalic{Offset: 0, Length: 5},
	})
	if result != "__world__" {
		t.Errorf("got %q, want %q", result, "__world__")
	}
}

func TestUnparseMarkdownCode(t *testing.T) {
	result := extensions.UnparseMarkdown("x=1", []tg.MessageEntityClass{
		&tg.MessageEntityCode{Offset: 0, Length: 3},
	})
	if result != "`x=1`" {
		t.Errorf("got %q, want %q", result, "`x=1`")
	}
}

func TestUnparseMarkdownPre(t *testing.T) {
	result := extensions.UnparseMarkdown("code", []tg.MessageEntityClass{
		&tg.MessageEntityPre{Offset: 0, Length: 4},
	})
	if result != "```code```" {
		t.Errorf("got %q, want %q", result, "```code```")
	}
}

func TestUnparseMarkdownStrike(t *testing.T) {
	result := extensions.UnparseMarkdown("del", []tg.MessageEntityClass{
		&tg.MessageEntityStrike{Offset: 0, Length: 3},
	})
	if result != "~~del~~" {
		t.Errorf("got %q, want %q", result, "~~del~~")
	}
}

func TestUnparseMarkdownTextURL(t *testing.T) {
	result := extensions.UnparseMarkdown("click", []tg.MessageEntityClass{
		&tg.MessageEntityTextURL{Offset: 0, Length: 5, URL: "https://t.me"},
	})
	want := "[click](https://t.me)"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseMarkdownMention(t *testing.T) {
	result := extensions.UnparseMarkdown("name", []tg.MessageEntityClass{
		&tg.MessageEntityMentionName{Offset: 0, Length: 4, UserID: 42},
	})
	want := "[name](tg://user?id=42)"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseMarkdownNoEntities(t *testing.T) {
	result := extensions.UnparseMarkdown("plain text", nil)
	if result != "plain text" {
		t.Errorf("got %q, want %q", result, "plain text")
	}
}

func TestUnparseMarkdownEmpty(t *testing.T) {
	result := extensions.UnparseMarkdown("", nil)
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Round-trip tests
// ─────────────────────────────────────────────────────────────────────────────

func TestParseMarkdownRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		md   string
	}{
		{"bold", "**bold**"},
		{"italic", "__italic__"},
		{"strike", "~~strike~~"},
		{"code", "`code`"},
		{"pre", "```pre```"},
		{"url", "[text](https://t.me)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, entities, err := extensions.ParseMarkdown(tc.md)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.md, err)
			}
			result := extensions.UnparseMarkdown(text, entities)
			// Re-parse the result.
			text2, entities2, err2 := extensions.ParseMarkdown(result)
			if err2 != nil {
				t.Fatalf("re-parse %q: %v", result, err2)
			}
			if text != text2 {
				t.Errorf("text mismatch: %q vs %q", text, text2)
			}
			if len(entities) != len(entities2) {
				t.Errorf("entity count mismatch for %q: %d vs %d (unparse=%q)",
					tc.md, len(entities), len(entities2), result)
			}
		})
	}
}

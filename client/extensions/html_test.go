package extensions_test

import (
	"testing"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/client/extensions"
)

// ─────────────────────────────────────────────────────────────────────────────
// ParseHTML
// ─────────────────────────────────────────────────────────────────────────────

func TestParseHTMLBold(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<b>hello</b>")
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

func TestParseHTMLStrong(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<strong>world</strong>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "world" {
		t.Errorf("text: got %q, want %q", text, "world")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityBold); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityBold", entities[0])
	}
}

func TestParseHTMLItalic(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<i>italic</i>")
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

func TestParseHTMLEm(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<em>em</em>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "em" {
		t.Errorf("text: got %q, want %q", text, "em")
	}
	if _, ok := entities[0].(*tg.MessageEntityItalic); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityItalic", entities[0])
	}
}

func TestParseHTMLCode(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<code>x = 1</code>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "x = 1" {
		t.Errorf("text: got %q, want %q", text, "x = 1")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	code, ok := entities[0].(*tg.MessageEntityCode)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityCode", entities[0])
	}
	if code.Offset != 0 || code.Length != 5 {
		t.Errorf("offset/length: got %d/%d, want 0/5", code.Offset, code.Length)
	}
}

func TestParseHTMLPre(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<pre>code block</pre>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "code block" {
		t.Errorf("text: got %q, want %q", text, "code block")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	pre, ok := entities[0].(*tg.MessageEntityPre)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityPre", entities[0])
	}
	if pre.Language != "" {
		t.Errorf("language: got %q, want %q", pre.Language, "")
	}
	if pre.Offset != 0 || pre.Length != 10 {
		t.Errorf("offset/length: got %d/%d, want 0/10", pre.Offset, pre.Length)
	}
}

func TestParseHTMLPreWithLanguage(t *testing.T) {
	text, entities, err := extensions.ParseHTML(`<pre><code class="language-python">x=1</code></pre>`)
	if err != nil {
		t.Fatal(err)
	}
	if text != "x=1" {
		t.Errorf("text: got %q, want %q", text, "x=1")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	pre, ok := entities[0].(*tg.MessageEntityPre)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityPre", entities[0])
	}
	if pre.Language != "python" {
		t.Errorf("language: got %q, want %q", pre.Language, "python")
	}
}

func TestParseHTMLUnderline(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<u>under</u>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "under" {
		t.Errorf("text: got %q, want %q", text, "under")
	}
	if _, ok := entities[0].(*tg.MessageEntityUnderline); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityUnderline", entities[0])
	}
}

func TestParseHTMLStrike(t *testing.T) {
	for _, tag := range []string{"del", "s"} {
		html := "<" + tag + ">strike</" + tag + ">"
		text, entities, err := extensions.ParseHTML(html)
		if err != nil {
			t.Fatalf("tag %s: %v", tag, err)
		}
		if text != "strike" {
			t.Errorf("tag %s: text got %q, want %q", tag, text, "strike")
		}
		if _, ok := entities[0].(*tg.MessageEntityStrike); !ok {
			t.Errorf("tag %s: entity type got %T, want *tg.MessageEntityStrike", tag, entities[0])
		}
	}
}

func TestParseHTMLLink(t *testing.T) {
	text, entities, err := extensions.ParseHTML(`<a href="https://t.me">link</a>`)
	if err != nil {
		t.Fatal(err)
	}
	if text != "link" {
		t.Errorf("text: got %q, want %q", text, "link")
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

// When href == text, it should be downgraded to MessageEntityURL.
func TestParseHTMLLinkAutoURL(t *testing.T) {
	url := "https://example.com"
	html := `<a href="` + url + `">` + url + `</a>`
	_, entities, err := extensions.ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityURL); !ok {
		t.Errorf("entity type: got %T, want *tg.MessageEntityURL (auto-downgrade)", entities[0])
	}
}

func TestParseHTMLMentionLink(t *testing.T) {
	text, entities, err := extensions.ParseHTML(`<a href="tg://user?id=123">name</a>`)
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

func TestParseHTMLEmail(t *testing.T) {
	text, entities, err := extensions.ParseHTML(`<a href="mailto:x@y.z">x@y.z</a>`)
	if err != nil {
		t.Fatal(err)
	}
	if text != "x@y.z" {
		t.Errorf("text: got %q, want %q", text, "x@y.z")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityEmail); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityEmail", entities[0])
	}
}

func TestParseHTMLTGEmoji(t *testing.T) {
	// 🔮 is U+1F52E — 2 UTF-16 code units.
	text, entities, err := extensions.ParseHTML(`<tg-emoji emoji-id="5368324170671202286">🔮</tg-emoji>`)
	if err != nil {
		t.Fatal(err)
	}
	if text != "🔮" {
		t.Errorf("text: got %q, want %q", text, "🔮")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	ce, ok := entities[0].(*tg.MessageEntityCustomEmoji)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityCustomEmoji", entities[0])
	}
	if ce.DocumentID != 5368324170671202286 {
		t.Errorf("DocumentID: got %d, want 5368324170671202286", ce.DocumentID)
	}
	// 🔮 occupies 2 UTF-16 code units.
	if ce.Length != 2 {
		t.Errorf("length: got %d, want 2 (emoji = 2 UTF-16 units)", ce.Length)
	}
}

func TestParseHTMLSpoiler(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<tg-spoiler>hidden</tg-spoiler>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hidden" {
		t.Errorf("text: got %q, want %q", text, "hidden")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntitySpoiler); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntitySpoiler", entities[0])
	}
}

func TestParseHTMLBlockquote(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<blockquote>quote</blockquote>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "quote" {
		t.Errorf("text: got %q, want %q", text, "quote")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	bq, ok := entities[0].(*tg.MessageEntityBlockquote)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityBlockquote", entities[0])
	}
	if bq.Offset != 0 || bq.Length != 5 {
		t.Errorf("offset/length: got %d/%d, want 0/5", bq.Offset, bq.Length)
	}
}

// The expandable attribute is recorded but gotd v0.89.0 has no Collapsed field.
// We verify parsing succeeds and returns a blockquote entity.
func TestParseHTMLBlockquoteExpandable(t *testing.T) {
	text, entities, err := extensions.ParseHTML("<blockquote expandable>collapsible</blockquote>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "collapsible" {
		t.Errorf("text: got %q, want %q", text, "collapsible")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	if _, ok := entities[0].(*tg.MessageEntityBlockquote); !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityBlockquote", entities[0])
	}
}

func TestParseHTMLNested(t *testing.T) {
	// <b><i>bold italic</i></b> → two entities covering the same range.
	text, entities, err := extensions.ParseHTML("<b><i>bold italic</i></b>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "bold italic" {
		t.Errorf("text: got %q, want %q", text, "bold italic")
	}
	if len(entities) != 2 {
		t.Fatalf("entity count: got %d, want 2", len(entities))
	}
	wantLen := 11 // "bold italic"
	for _, e := range entities {
		switch v := e.(type) {
		case *tg.MessageEntityBold:
			if v.Offset != 0 || v.Length != wantLen {
				t.Errorf("bold offset/length: got %d/%d, want 0/%d", v.Offset, v.Length, wantLen)
			}
		case *tg.MessageEntityItalic:
			if v.Offset != 0 || v.Length != wantLen {
				t.Errorf("italic offset/length: got %d/%d, want 0/%d", v.Offset, v.Length, wantLen)
			}
		default:
			t.Errorf("unexpected entity type: %T", e)
		}
	}
}

func TestParseHTMLEmojiOffset(t *testing.T) {
	// "hi 🔮 " = 3 BMP + 2 SMP + 1 BMP = 6 UTF-16 units before "bold".
	text, entities, err := extensions.ParseHTML("hi 🔮 <b>bold</b>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "hi 🔮 bold" {
		t.Errorf("text: got %q, want %q", text, "hi 🔮 bold")
	}
	if len(entities) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(entities))
	}
	bold, ok := entities[0].(*tg.MessageEntityBold)
	if !ok {
		t.Fatalf("entity type: got %T, want *tg.MessageEntityBold", entities[0])
	}
	// h(0) i(1) sp(2) 🔮(3,4) sp(5) → "bold" starts at UTF-16 offset 6
	if bold.Offset != 6 {
		t.Errorf("bold offset: got %d, want 6 (emoji counted as 2 UTF-16 units)", bold.Offset)
	}
	if bold.Length != 4 {
		t.Errorf("bold length: got %d, want 4", bold.Length)
	}
}

func TestParseHTMLEmpty(t *testing.T) {
	text, entities, err := extensions.ParseHTML("")
	if err != nil {
		t.Fatal(err)
	}
	if text != "" {
		t.Errorf("text: got %q, want %q", text, "")
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0", len(entities))
	}
}

func TestParseHTMLPlainText(t *testing.T) {
	text, entities, err := extensions.ParseHTML("hello world")
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

func TestParseHTMLUnsupportedTagPreserved(t *testing.T) {
	// Unknown tags are kept literally in the text output.
	text, entities, err := extensions.ParseHTML("<div>hello</div>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "<div>hello</div>" {
		t.Errorf("text: got %q, want literal tag preserved", text)
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0 (no entities for unknown tags)", len(entities))
	}
}

func TestParseHTMLInvalidTagSyntaxEscaped(t *testing.T) {
	// "<3" is not a valid tag and should be kept as text.
	text, entities, err := extensions.ParseHTML("Hello <3 world")
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0", len(entities))
	}
	if text == "" {
		t.Error("text should not be empty")
	}
}

func TestParseHTMLHTMLEscapedEntities(t *testing.T) {
	text, _, err := extensions.ParseHTML("<b>a &amp; b</b>")
	if err != nil {
		t.Fatal(err)
	}
	if text != "a & b" {
		t.Errorf("text: got %q, want %q", text, "a & b")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UnparseHTML
// ─────────────────────────────────────────────────────────────────────────────

func TestUnparseHTMLBold(t *testing.T) {
	result := extensions.UnparseHTML("hello", []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: 5},
	})
	if result != "<strong>hello</strong>" {
		t.Errorf("got %q, want %q", result, "<strong>hello</strong>")
	}
}

func TestUnparseHTMLItalic(t *testing.T) {
	result := extensions.UnparseHTML("hi", []tg.MessageEntityClass{
		&tg.MessageEntityItalic{Offset: 0, Length: 2},
	})
	if result != "<em>hi</em>" {
		t.Errorf("got %q, want %q", result, "<em>hi</em>")
	}
}

func TestUnparseHTMLCode(t *testing.T) {
	result := extensions.UnparseHTML("x=1", []tg.MessageEntityClass{
		&tg.MessageEntityCode{Offset: 0, Length: 3},
	})
	if result != "<code>x=1</code>" {
		t.Errorf("got %q, want %q", result, "<code>x=1</code>")
	}
}

func TestUnparseHTMLPre(t *testing.T) {
	result := extensions.UnparseHTML("code", []tg.MessageEntityClass{
		&tg.MessageEntityPre{Offset: 0, Length: 4, Language: ""},
	})
	if result != "<pre><code>code</code></pre>" {
		t.Errorf("got %q, want %q", result, "<pre><code>code</code></pre>")
	}
}

func TestUnparseHTMLPreWithLanguage(t *testing.T) {
	result := extensions.UnparseHTML("x=1", []tg.MessageEntityClass{
		&tg.MessageEntityPre{Offset: 0, Length: 3, Language: "python"},
	})
	want := "<pre><code class='language-python'>x=1</code></pre>"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseHTMLTextURL(t *testing.T) {
	result := extensions.UnparseHTML("link", []tg.MessageEntityClass{
		&tg.MessageEntityTextURL{Offset: 0, Length: 4, URL: "https://t.me"},
	})
	want := `<a href="https://t.me">link</a>`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseHTMLMentionName(t *testing.T) {
	result := extensions.UnparseHTML("name", []tg.MessageEntityClass{
		&tg.MessageEntityMentionName{Offset: 0, Length: 4, UserID: 123},
	})
	want := `<a href="tg://user?id=123">name</a>`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseHTMLSpoiler(t *testing.T) {
	result := extensions.UnparseHTML("secret", []tg.MessageEntityClass{
		&tg.MessageEntitySpoiler{Offset: 0, Length: 6},
	})
	if result != "<tg-spoiler>secret</tg-spoiler>" {
		t.Errorf("got %q", result)
	}
}

func TestUnparseHTMLBlockquote(t *testing.T) {
	result := extensions.UnparseHTML("quote", []tg.MessageEntityClass{
		&tg.MessageEntityBlockquote{Offset: 0, Length: 5},
	})
	if result != "<blockquote>quote</blockquote>" {
		t.Errorf("got %q", result)
	}
}

func TestUnparseHTMLCustomEmoji(t *testing.T) {
	result := extensions.UnparseHTML("🔮", []tg.MessageEntityClass{
		&tg.MessageEntityCustomEmoji{Offset: 0, Length: 2, DocumentID: 123},
	})
	want := `<tg-emoji emoji-id="123">🔮</tg-emoji>`
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseHTMLNoEntities(t *testing.T) {
	result := extensions.UnparseHTML("hello world", nil)
	if result != "hello world" {
		t.Errorf("got %q, want %q", result, "hello world")
	}
}

func TestUnparseHTMLEmpty(t *testing.T) {
	result := extensions.UnparseHTML("", nil)
	if result != "" {
		t.Errorf("got %q, want empty", result)
	}
}

func TestUnparseHTMLEscapesSpecialChars(t *testing.T) {
	result := extensions.UnparseHTML("a & b < c > d", nil)
	want := "a &amp; b &lt; c &gt; d"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestUnparseHTMLMidText(t *testing.T) {
	result := extensions.UnparseHTML("hello world!", []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 6, Length: 5},
	})
	want := "hello <strong>world</strong>!"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Round-trip tests
// ─────────────────────────────────────────────────────────────────────────────

func TestUnparseHTMLRoundTrip(t *testing.T) {
	cases := []struct {
		name string
		html string
	}{
		{"bold-b", "<b>bold</b>"},
		{"italic-i", "<i>italic</i>"},
		{"code", "<code>code</code>"},
		{"plain-with-bold", "hello <b>world</b>!"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			text, entities, err := extensions.ParseHTML(tc.html)
			if err != nil {
				t.Fatalf("parse %q: %v", tc.html, err)
			}
			result := extensions.UnparseHTML(text, entities)
			// Re-parse the result — should yield the same text and entity count.
			text2, entities2, err2 := extensions.ParseHTML(result)
			if err2 != nil {
				t.Fatalf("re-parse %q: %v", result, err2)
			}
			if text != text2 {
				t.Errorf("text mismatch: %q vs %q", text, text2)
			}
			if len(entities) != len(entities2) {
				t.Errorf("entity count mismatch: %d vs %d", len(entities), len(entities2))
			}
		})
	}
}

func TestUnparseHTMLRoundTripNested(t *testing.T) {
	// Parse then unparse then parse again — entity counts must match.
	html := "<b><i>bold italic</i></b>"
	text, entities, err := extensions.ParseHTML(html)
	if err != nil {
		t.Fatal(err)
	}
	result := extensions.UnparseHTML(text, entities)
	_, entities2, err2 := extensions.ParseHTML(result)
	if err2 != nil {
		t.Fatalf("re-parse: %v", err2)
	}
	if len(entities) != len(entities2) {
		t.Errorf("entity count mismatch after round-trip: %d vs %d", len(entities), len(entities2))
	}
}

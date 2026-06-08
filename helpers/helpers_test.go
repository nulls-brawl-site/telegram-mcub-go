package helpers_test

import (
	"testing"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/helpers"
)

// ─────────────────────────────────────────────────────────────────────────────
// UTF16Len
// ─────────────────────────────────────────────────────────────────────────────

func TestUTF16Len(t *testing.T) {
	cases := []struct {
		s    string
		want int
	}{
		{"hello", 5},
		{"🔮", 2},       // U+1F52E — SMP, 2 UTF-16 code units
		{"hi 🔮", 5},    // "hi " (3) + emoji (2)
		{"", 0},
		{"a", 1},
		{"🌍🌎🌏", 6}, // three emoji × 2
		{"abc def", 7},
		// BMP emoji (U+2764, ❤) costs 1 unit.
		{"❤", 1},
	}
	for _, c := range cases {
		got := helpers.UTF16Len(c.s)
		if got != c.want {
			t.Errorf("UTF16Len(%q) = %d, want %d", c.s, got, c.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// UTF16RuneLen
// ─────────────────────────────────────────────────────────────────────────────

func TestUTF16RuneLen(t *testing.T) {
	cases := []struct {
		r    rune
		want int
	}{
		{'a', 1},
		{'Z', 1},
		{'€', 1},           // U+20AC — BMP
		{'\U0001F52E', 2},  // 🔮 — SMP
		{'\U0001F600', 2},  // 😀 — SMP
	}
	for _, c := range cases {
		got := helpers.UTF16RuneLen(c.r)
		if got != c.want {
			t.Errorf("UTF16RuneLen(%U) = %d, want %d", c.r, got, c.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ToUTF16 / FromUTF16 round-trip
// ─────────────────────────────────────────────────────────────────────────────

func TestToFromUTF16RoundTrip(t *testing.T) {
	cases := []string{
		"hello",
		"hello 🔮 world",
		"",
		"a",
		"🔮🌍",
	}
	for _, s := range cases {
		u16 := helpers.ToUTF16(s)
		got := helpers.FromUTF16(u16)
		if got != s {
			t.Errorf("round-trip(%q): got %q", s, got)
		}
	}
}

func TestToUTF16LenMatchesUTF16Len(t *testing.T) {
	cases := []string{"hello", "🔮", "hi 🔮", "abc🌍def"}
	for _, s := range cases {
		u16 := helpers.ToUTF16(s)
		if len(u16) != helpers.UTF16Len(s) {
			t.Errorf("ToUTF16 len(%q): got %d, UTF16Len gives %d", s, len(u16), helpers.UTF16Len(s))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// WithinSurrogate
// ─────────────────────────────────────────────────────────────────────────────

func TestWithinSurrogate(t *testing.T) {
	// "🔮" encodes to [0xD83D, 0xDD2E].
	// Position 1 is the low surrogate — within the pair → true.
	// Position 0 is the high surrogate — not "within" (it's the start) → false.
	u16 := helpers.ToUTF16("🔮")

	if !helpers.WithinSurrogate(u16, 1) {
		t.Error("WithinSurrogate(🔮, 1) should be true (low surrogate at position 1)")
	}
	if helpers.WithinSurrogate(u16, 0) {
		t.Error("WithinSurrogate(🔮, 0) should be false (i <= 0 boundary)")
	}
}

func TestWithinSurrogateTrue(t *testing.T) {
	// "a🔮b": u16 = [a, 0xD83D, 0xDD2E, b]
	// Position 2 is within the surrogate pair (high at 1, low at 2).
	u16 := helpers.ToUTF16("a🔮b")
	if !helpers.WithinSurrogate(u16, 2) {
		t.Error("WithinSurrogate(a🔮b, 2) should be true")
	}
	if helpers.WithinSurrogate(u16, 1) {
		t.Error("WithinSurrogate(a🔮b, 1) should be false (it's the high surrogate)")
	}
	if helpers.WithinSurrogate(u16, 3) {
		t.Error("WithinSurrogate(a🔮b, 3) should be false")
	}
}

func TestWithinSurrogateBoundary(t *testing.T) {
	u16 := helpers.ToUTF16("hello")
	// No surrogates in ASCII text.
	for i := 0; i <= len(u16); i++ {
		if helpers.WithinSurrogate(u16, i) {
			t.Errorf("WithinSurrogate(hello, %d) should be false", i)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetEntityOffset / GetEntityLength / SetEntityOffset / SetEntityLength
// ─────────────────────────────────────────────────────────────────────────────

func TestGetSetEntityBold(t *testing.T) {
	e := &tg.MessageEntityBold{Offset: 5, Length: 10}
	if helpers.GetEntityOffset(e) != 5 {
		t.Errorf("GetEntityOffset: got %d, want 5", helpers.GetEntityOffset(e))
	}
	if helpers.GetEntityLength(e) != 10 {
		t.Errorf("GetEntityLength: got %d, want 10", helpers.GetEntityLength(e))
	}
	helpers.SetEntityOffset(e, 3)
	helpers.SetEntityLength(e, 7)
	if e.Offset != 3 || e.Length != 7 {
		t.Errorf("after set: Offset=%d Length=%d, want 3/7", e.Offset, e.Length)
	}
}

func TestGetSetEntityAllTypes(t *testing.T) {
	entities := []tg.MessageEntityClass{
		&tg.MessageEntityItalic{Offset: 1, Length: 2},
		&tg.MessageEntityCode{Offset: 3, Length: 4},
		&tg.MessageEntityPre{Offset: 5, Length: 6},
		&tg.MessageEntityTextURL{Offset: 7, Length: 8},
		&tg.MessageEntityMentionName{Offset: 9, Length: 10},
		&tg.MessageEntityStrike{Offset: 11, Length: 12},
		&tg.MessageEntitySpoiler{Offset: 13, Length: 14},
		&tg.MessageEntityUnderline{Offset: 15, Length: 16},
		&tg.MessageEntityBlockquote{Offset: 17, Length: 18},
		&tg.MessageEntityCustomEmoji{Offset: 19, Length: 20},
		&tg.MessageEntityURL{Offset: 21, Length: 22},
		&tg.MessageEntityEmail{Offset: 23, Length: 24},
		&tg.MessageEntityMention{Offset: 25, Length: 26},
	}
	for i, e := range entities {
		wantOff := 2*i + 1
		wantLen := 2*i + 2
		if off := helpers.GetEntityOffset(e); off != wantOff {
			t.Errorf("%T GetEntityOffset: got %d, want %d", e, off, wantOff)
		}
		if l := helpers.GetEntityLength(e); l != wantLen {
			t.Errorf("%T GetEntityLength: got %d, want %d", e, l, wantLen)
		}
		helpers.SetEntityOffset(e, 100)
		helpers.SetEntityLength(e, 200)
		if off := helpers.GetEntityOffset(e); off != 100 {
			t.Errorf("%T SetEntityOffset: got %d, want 100", e, off)
		}
		if l := helpers.GetEntityLength(e); l != 200 {
			t.Errorf("%T SetEntityLength: got %d, want 200", e, l)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// StripText
// ─────────────────────────────────────────────────────────────────────────────

func TestStripTextNoEntities(t *testing.T) {
	text, entities := helpers.StripText("  hello world  ", nil)
	if text != "hello world" {
		t.Errorf("got %q, want %q", text, "hello world")
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0", len(entities))
	}
}

func TestStripTextLeadingWhitespace(t *testing.T) {
	// Entity starting after leading whitespace: offset should shift left.
	// "  bold" → strip → "bold"; bold entity at original offset 2 → new offset 0.
	ents := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 2, Length: 4},
	}
	text, out := helpers.StripText("  bold", ents)
	if text != "bold" {
		t.Errorf("text: got %q, want %q", text, "bold")
	}
	if len(out) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(out))
	}
	b := out[0].(*tg.MessageEntityBold)
	if b.Offset != 0 {
		t.Errorf("offset: got %d, want 0", b.Offset)
	}
	if b.Length != 4 {
		t.Errorf("length: got %d, want 4", b.Length)
	}
}

func TestStripTextTrailingWhitespace(t *testing.T) {
	// "bold  " → strip → "bold"; entity at 0/4 remains.
	ents := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: 4},
	}
	text, out := helpers.StripText("bold  ", ents)
	if text != "bold" {
		t.Errorf("text: got %q, want %q", text, "bold")
	}
	if len(out) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(out))
	}
	b := out[0].(*tg.MessageEntityBold)
	if b.Offset != 0 || b.Length != 4 {
		t.Errorf("offset/length: got %d/%d, want 0/4", b.Offset, b.Length)
	}
}

func TestStripTextEntityInWhitespace(t *testing.T) {
	// Entity entirely in leading whitespace → dropped.
	ents := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: 2},
	}
	text, out := helpers.StripText("  hello", ents)
	if text != "hello" {
		t.Errorf("text: got %q, want %q", text, "hello")
	}
	if len(out) != 0 {
		t.Errorf("entity count: got %d, want 0 (entity in stripped whitespace)", len(out))
	}
}

func TestStripTextZeroLengthEntityDropped(t *testing.T) {
	ents := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: 0},
	}
	_, out := helpers.StripText("hello", ents)
	if len(out) != 0 {
		t.Errorf("entity count: got %d, want 0 (zero-length entities are dropped)", len(out))
	}
}

func TestStripTextNoWhitespace(t *testing.T) {
	ents := []tg.MessageEntityClass{
		&tg.MessageEntityBold{Offset: 0, Length: 5},
	}
	text, out := helpers.StripText("hello", ents)
	if text != "hello" {
		t.Errorf("text: got %q, want %q", text, "hello")
	}
	if len(out) != 1 {
		t.Fatalf("entity count: got %d, want 1", len(out))
	}
}

func TestStripTextEmptyString(t *testing.T) {
	text, entities := helpers.StripText("", nil)
	if text != "" {
		t.Errorf("got %q, want empty", text)
	}
	if len(entities) != 0 {
		t.Errorf("entity count: got %d, want 0", len(entities))
	}
}

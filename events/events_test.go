package events_test

import (
	"strings"
	"testing"

	"github.com/gotd/td/tg"

	"github.com/nulls-brawl-site/telegram-mcub-go/events"
)

// helper: build a NewMessage with the given text and optional overrides.
func newMsg(text string) *events.NewMessage {
	return &events.NewMessage{
		Raw: &tg.Message{Message: text},
	}
}

func newMsgWithSender(text string, senderID, peerID int64) *events.NewMessage {
	return &events.NewMessage{
		Raw:      &tg.Message{Message: text},
		SenderID: senderID,
		PeerID:   peerID,
	}
}

// ---------------------------------------------------------------------------
// PatternFilter (existing)
// ---------------------------------------------------------------------------

func TestPatternFilter(t *testing.T) {
	f := events.PatternFilter(func(t string) bool { return strings.HasPrefix(t, ".ping") })

	ev := newMsg(".ping hello")
	if !f(ev) {
		t.Fatal("expected match for '.ping hello'")
	}

	ev2 := newMsg(".pong")
	if f(ev2) {
		t.Fatal("expected no match for '.pong'")
	}

	// Non-NewMessage event should not match.
	raw := &events.Raw{Update: nil}
	if f(raw) {
		t.Fatal("expected no match for non-NewMessage event")
	}
}

// ---------------------------------------------------------------------------
// RegexFilter
// ---------------------------------------------------------------------------

func TestRegexFilter(t *testing.T) {
	f := events.RegexFilter(`^\.ping`)

	if !f(newMsg(".ping args")) {
		t.Fatal("regex should match '.ping args'")
	}
	if !f(newMsg(".ping")) {
		t.Fatal("regex should match '.ping'")
	}
	if f(newMsg("prefix .ping")) {
		t.Fatal("regex should not match 'prefix .ping' (anchored)")
	}
	if f(&events.Raw{}) {
		t.Fatal("regex filter should not match non-NewMessage")
	}
}

// ---------------------------------------------------------------------------
// CommandFilter
// ---------------------------------------------------------------------------

func TestCommandFilter(t *testing.T) {
	f := events.CommandFilter("/", "start")

	if !f(newMsg("/start")) {
		t.Fatal("should match exact command '/start'")
	}
	if !f(newMsg("/start arg1 arg2")) {
		t.Fatal("should match command with args '/start arg1 arg2'")
	}
	if f(newMsg("/starting")) {
		t.Fatal("should not match '/starting' (partial prefix)")
	}
	if f(newMsg("start")) {
		t.Fatal("should not match 'start' (missing prefix)")
	}
	if f(&events.Raw{}) {
		t.Fatal("should not match non-NewMessage")
	}
}

// ---------------------------------------------------------------------------
// ChatsFilter
// ---------------------------------------------------------------------------

func TestChatsFilter(t *testing.T) {
	f := events.ChatsFilter(100, 200)

	ev := newMsgWithSender("hi", 1, 100)
	if !f(ev) {
		t.Fatal("should match peerID=100")
	}

	ev2 := newMsgWithSender("hi", 1, 300)
	if f(ev2) {
		t.Fatal("should not match peerID=300")
	}

	// MessageEdited should also be filtered.
	edited := &events.MessageEdited{Raw: &tg.Message{}, PeerID: 200}
	if !f(edited) {
		t.Fatal("should match edited message with peerID=200")
	}

	edited2 := &events.MessageEdited{Raw: &tg.Message{}, PeerID: 999}
	if f(edited2) {
		t.Fatal("should not match edited message with peerID=999")
	}
}

// ---------------------------------------------------------------------------
// FromUsersFilter
// ---------------------------------------------------------------------------

func TestFromUsersFilter(t *testing.T) {
	f := events.FromUsersFilter(42, 99)

	if !f(newMsgWithSender("hi", 42, 0)) {
		t.Fatal("should match senderID=42")
	}
	if f(newMsgWithSender("hi", 1, 0)) {
		t.Fatal("should not match senderID=1")
	}
	if f(&events.Raw{}) {
		t.Fatal("should not match non-NewMessage")
	}
}

// ---------------------------------------------------------------------------
// OutgoingFilter / IncomingFilter
// ---------------------------------------------------------------------------

func TestOutgoingFilter(t *testing.T) {
	f := events.OutgoingFilter()

	out := &events.NewMessage{Raw: &tg.Message{Message: "out"}, IsOutgoing: true}
	in := &events.NewMessage{Raw: &tg.Message{Message: "in"}, IsOutgoing: false}

	if !f(out) {
		t.Fatal("OutgoingFilter should match outgoing message")
	}
	if f(in) {
		t.Fatal("OutgoingFilter should not match incoming message")
	}
}

func TestIncomingFilter(t *testing.T) {
	f := events.IncomingFilter()

	out := &events.NewMessage{Raw: &tg.Message{Message: "out"}, IsOutgoing: true}
	in := &events.NewMessage{Raw: &tg.Message{Message: "in"}, IsOutgoing: false}

	if f(out) {
		t.Fatal("IncomingFilter should not match outgoing message")
	}
	if !f(in) {
		t.Fatal("IncomingFilter should match incoming message")
	}
}

// ---------------------------------------------------------------------------
// PrivateFilter / GroupFilter / ChannelFilter
// ---------------------------------------------------------------------------

func TestPrivateFilter(t *testing.T) {
	f := events.PrivateFilter()

	priv := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true}
	grp := &events.NewMessage{Raw: &tg.Message{}, IsGroup: true}

	if !f(priv) {
		t.Fatal("PrivateFilter should match private message")
	}
	if f(grp) {
		t.Fatal("PrivateFilter should not match group message")
	}
}

func TestGroupFilter(t *testing.T) {
	f := events.GroupFilter()

	grp := &events.NewMessage{Raw: &tg.Message{}, IsGroup: true}
	priv := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true}

	if !f(grp) {
		t.Fatal("GroupFilter should match group message")
	}
	if f(priv) {
		t.Fatal("GroupFilter should not match private message")
	}
}

func TestChannelFilter(t *testing.T) {
	f := events.ChannelFilter()

	ch := &events.NewMessage{Raw: &tg.Message{}, IsChannel: true}
	priv := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true}

	if !f(ch) {
		t.Fatal("ChannelFilter should match channel message")
	}
	if f(priv) {
		t.Fatal("ChannelFilter should not match private message")
	}
}

// ---------------------------------------------------------------------------
// HasMediaFilter
// ---------------------------------------------------------------------------

func TestHasMediaFilter(t *testing.T) {
	f := events.HasMediaFilter()

	withMedia := &events.NewMessage{
		Raw: &tg.Message{Media: &tg.MessageMediaPhoto{}},
	}
	noMedia := &events.NewMessage{
		Raw: &tg.Message{},
	}

	if !f(withMedia) {
		t.Fatal("HasMediaFilter should match message with media")
	}
	if f(noMedia) {
		t.Fatal("HasMediaFilter should not match message without media")
	}
	if f(&events.Raw{}) {
		t.Fatal("HasMediaFilter should not match non-NewMessage")
	}
}

// ---------------------------------------------------------------------------
// MediaTypeFilter
// ---------------------------------------------------------------------------

func TestMediaTypeFilter(t *testing.T) {
	fPhoto := events.MediaTypeFilter("photo")
	fDoc := events.MediaTypeFilter("document")
	fLoc := events.MediaTypeFilter("location")

	photo := &events.NewMessage{Raw: &tg.Message{Media: &tg.MessageMediaPhoto{}}}
	doc := &events.NewMessage{Raw: &tg.Message{Media: &tg.MessageMediaDocument{}}}
	geo := &events.NewMessage{Raw: &tg.Message{Media: &tg.MessageMediaGeo{}}}
	noMedia := &events.NewMessage{Raw: &tg.Message{}}

	if !fPhoto(photo) {
		t.Fatal("photo filter should match photo media")
	}
	if fPhoto(doc) {
		t.Fatal("photo filter should not match document media")
	}
	if !fDoc(doc) {
		t.Fatal("document filter should match document media")
	}
	if !fLoc(geo) {
		t.Fatal("location filter should match geo media")
	}
	if fPhoto(noMedia) {
		t.Fatal("photo filter should not match message without media")
	}
}

// ---------------------------------------------------------------------------
// AndFilter / OrFilter / NotFilter
// ---------------------------------------------------------------------------

func TestAndFilter(t *testing.T) {
	isPrivate := events.PrivateFilter()
	isIncoming := events.IncomingFilter()
	f := events.AndFilter(isPrivate, isIncoming)

	both := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true, IsOutgoing: false}
	onlyPrivate := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true, IsOutgoing: true}
	neither := &events.NewMessage{Raw: &tg.Message{}, IsGroup: true, IsOutgoing: false}

	if !f(both) {
		t.Fatal("AndFilter should match when all filters pass")
	}
	if f(onlyPrivate) {
		t.Fatal("AndFilter should not match when only one filter passes")
	}
	if f(neither) {
		t.Fatal("AndFilter should not match when no filters pass")
	}
}

func TestOrFilter(t *testing.T) {
	isPrivate := events.PrivateFilter()
	isGroup := events.GroupFilter()
	f := events.OrFilter(isPrivate, isGroup)

	priv := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true}
	grp := &events.NewMessage{Raw: &tg.Message{}, IsGroup: true}
	ch := &events.NewMessage{Raw: &tg.Message{}, IsChannel: true}

	if !f(priv) {
		t.Fatal("OrFilter should match private message")
	}
	if !f(grp) {
		t.Fatal("OrFilter should match group message")
	}
	if f(ch) {
		t.Fatal("OrFilter should not match channel message (neither filter matches)")
	}
}

func TestNotFilter(t *testing.T) {
	f := events.NotFilter(events.PrivateFilter())

	priv := &events.NewMessage{Raw: &tg.Message{}, IsPrivate: true}
	grp := &events.NewMessage{Raw: &tg.Message{}, IsGroup: true}

	if f(priv) {
		t.Fatal("NotFilter should reject private message")
	}
	if !f(grp) {
		t.Fatal("NotFilter should accept non-private message")
	}
}

// ---------------------------------------------------------------------------
// NewMessage helper methods
// ---------------------------------------------------------------------------

func TestArgs(t *testing.T) {
	ev := newMsg("/start hello world")
	if ev.Args() != "hello world" {
		t.Fatalf("Args() = %q, want %q", ev.Args(), "hello world")
	}

	ev2 := newMsg("/start")
	if ev2.Args() != "" {
		t.Fatalf("Args() = %q, want empty string", ev2.Args())
	}
}

func TestArgsList(t *testing.T) {
	ev := newMsg("/cmd a b c")
	got := ev.ArgsList()
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("ArgsList() len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ArgsList()[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	ev2 := newMsg("/cmd")
	if ev2.ArgsList() != nil {
		t.Fatal("ArgsList() should be nil when no args")
	}
}

// ---------------------------------------------------------------------------
// BotInlineSend
// ---------------------------------------------------------------------------

func TestBotInlineSendFromUpdate(t *testing.T) {
	raw := &tg.UpdateBotInlineSend{
		UserID: 777,
		Query:  "test query",
		ID:     "result-1",
	}
	ev, ok := events.BotInlineSendFromUpdate(nil, raw)
	if !ok {
		t.Fatal("expected BotInlineSendFromUpdate to succeed")
	}
	if ev.UserID != 777 {
		t.Fatalf("UserID = %d, want 777", ev.UserID)
	}
	if ev.Query != "test query" {
		t.Fatalf("Query = %q, want 'test query'", ev.Query)
	}
	if ev.ResultID != "result-1" {
		t.Fatalf("ResultID = %q, want 'result-1'", ev.ResultID)
	}
	if ev.EventType() != "BotInlineSend" {
		t.Fatalf("EventType() = %q, want 'BotInlineSend'", ev.EventType())
	}
}

func TestBotInlineSendFromUpdateMismatch(t *testing.T) {
	_, ok := events.BotInlineSendFromUpdate(nil, &tg.UpdateNewMessage{
		Message: &tg.Message{},
	})
	if ok {
		t.Fatal("expected BotInlineSendFromUpdate to fail for non-matching update")
	}
}

func TestBotInlineSendFilter(t *testing.T) {
	f := events.BotInlineSendFilter()

	raw := &tg.UpdateBotInlineSend{UserID: 1, Query: "q", ID: "r"}
	ev, _ := events.BotInlineSendFromUpdate(nil, raw)

	if !f(ev) {
		t.Fatal("BotInlineSendFilter should match BotInlineSend event")
	}
	if f(newMsg("test")) {
		t.Fatal("BotInlineSendFilter should not match NewMessage event")
	}
}

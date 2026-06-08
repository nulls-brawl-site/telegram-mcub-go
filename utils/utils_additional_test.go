package utils_test

import (
	"testing"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/utils"
)

// ---------------------------------------------------------------------------
// GetTLMessage
// ---------------------------------------------------------------------------

func TestGetTLMessageUpdateNewMessage(t *testing.T) {
	msg := &tg.Message{ID: 42, Message: "hello"}
	upd := &tg.UpdateNewMessage{Message: msg}
	got := utils.GetTLMessage(upd)
	if got == nil {
		t.Fatal("GetTLMessage returned nil, want *tg.Message")
	}
	if got.ID != 42 {
		t.Errorf("ID: got %d, want 42", got.ID)
	}
	if got.Message != "hello" {
		t.Errorf("Message: got %q, want %q", got.Message, "hello")
	}
}

func TestGetTLMessageUpdateEditMessage(t *testing.T) {
	msg := &tg.Message{ID: 99}
	got := utils.GetTLMessage(&tg.UpdateEditMessage{Message: msg})
	if got == nil || got.ID != 99 {
		t.Errorf("UpdateEditMessage: got %v", got)
	}
}

func TestGetTLMessageUpdateNewChannelMessage(t *testing.T) {
	msg := &tg.Message{ID: 7}
	got := utils.GetTLMessage(&tg.UpdateNewChannelMessage{Message: msg})
	if got == nil || got.ID != 7 {
		t.Errorf("UpdateNewChannelMessage: got %v", got)
	}
}

func TestGetTLMessageUpdateEditChannelMessage(t *testing.T) {
	msg := &tg.Message{ID: 55}
	got := utils.GetTLMessage(&tg.UpdateEditChannelMessage{Message: msg})
	if got == nil || got.ID != 55 {
		t.Errorf("UpdateEditChannelMessage: got %v", got)
	}
}

func TestGetTLMessageNonMessage(t *testing.T) {
	// An update that doesn't carry a regular message should return nil.
	got := utils.GetTLMessage(&tg.UpdateUserStatus{})
	if got != nil {
		t.Errorf("expected nil for UpdateUserStatus, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// ParseUsername
// ---------------------------------------------------------------------------

func TestParseUsername(t *testing.T) {
	cases := []struct {
		in       string
		wantUser string
		wantInv  bool
	}{
		{"@username", "username", false},
		{"https://t.me/username", "username", false},
		{"t.me/username", "username", false},
		{"username", "username", false},
		{"telegram", "telegram", false},
	}
	for _, c := range cases {
		got, inv := utils.ParseUsername(c.in)
		if got != c.wantUser {
			t.Errorf("ParseUsername(%q) username = %q, want %q", c.in, got, c.wantUser)
		}
		if inv != c.wantInv {
			t.Errorf("ParseUsername(%q) isInvite = %v, want %v", c.in, inv, c.wantInv)
		}
	}
}

func TestParseUsernameInvalid(t *testing.T) {
	// Single-character strings are not valid Telegram usernames.
	got, inv := utils.ParseUsername("x")
	if got != "" || inv {
		t.Errorf("ParseUsername(\"x\") = (%q, %v), want (\"\", false)", got, inv)
	}
}

// ---------------------------------------------------------------------------
// ParsePhone
// ---------------------------------------------------------------------------

func TestParsePhone(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"+1 (234) 567-8900", "12345678900"},
		{"79001234567", "79001234567"},
		{"+7-900-123-45-67", "79001234567"},
	}
	for _, c := range cases {
		got := utils.ParsePhone(c.in)
		if got != c.want {
			t.Errorf("ParsePhone(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParsePhoneInvalid(t *testing.T) {
	got := utils.ParsePhone("notaphone")
	if got != "" {
		t.Errorf("ParsePhone(\"notaphone\") = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// GetMessageGroupID
// ---------------------------------------------------------------------------

func TestGetMessageGroupID(t *testing.T) {
	msg := &tg.Message{ID: 1}
	// GroupedID is a conditional field — must use the setter to also set the flag.
	msg.SetGroupedID(98765)
	if id := utils.GetMessageGroupID(msg); id != 98765 {
		t.Errorf("grouped_id: got %d, want 98765", id)
	}
}

func TestGetMessageGroupIDZero(t *testing.T) {
	msg := &tg.Message{ID: 2}
	// GroupedID not set — should return 0.
	if id := utils.GetMessageGroupID(msg); id != 0 {
		t.Errorf("unset grouped_id: got %d, want 0", id)
	}
}

func TestGetMessageGroupIDNil(t *testing.T) {
	if id := utils.GetMessageGroupID(nil); id != 0 {
		t.Errorf("nil message: got %d, want 0", id)
	}
}

// ---------------------------------------------------------------------------
// IsAudio / IsVideo / IsRound / IsAnimated / IsVoice
// ---------------------------------------------------------------------------

func TestIsRoundTrue(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{RoundMessage: true},
		},
	}
	if !utils.IsRound(doc) {
		t.Error("IsRound: expected true for round video")
	}
}

func TestIsRoundFalse(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{RoundMessage: false},
		},
	}
	if utils.IsRound(doc) {
		t.Error("IsRound: expected false for non-round video")
	}
}

func TestIsAnimatedTrue(t *testing.T) {
	doc := &tg.Document{MimeType: "application/x-tgsticker"}
	if !utils.IsAnimated(doc) {
		t.Error("IsAnimated: expected true for .tgs document")
	}
}

func TestIsAnimatedFalse(t *testing.T) {
	doc := &tg.Document{MimeType: "video/mp4"}
	if utils.IsAnimated(doc) {
		t.Error("IsAnimated: expected false for mp4 document")
	}
}

// ---------------------------------------------------------------------------
// GetInputDocument
// ---------------------------------------------------------------------------

func TestGetInputDocumentDirect(t *testing.T) {
	doc := &tg.Document{ID: 111, AccessHash: 222, FileReference: []byte("ref")}
	id, err := utils.GetInputDocument(doc)
	if err != nil {
		t.Fatal(err)
	}
	if id.ID != 111 || id.AccessHash != 222 {
		t.Errorf("got ID=%d Hash=%d, want 111/222", id.ID, id.AccessHash)
	}
}

func TestGetInputDocumentFromMedia(t *testing.T) {
	doc := &tg.Document{ID: 333}
	media := &tg.MessageMediaDocument{Document: doc}
	id, err := utils.GetInputDocument(media)
	if err != nil {
		t.Fatal(err)
	}
	if id.ID != 333 {
		t.Errorf("got ID=%d, want 333", id.ID)
	}
}

// ---------------------------------------------------------------------------
// GetInputPhoto
// ---------------------------------------------------------------------------

func TestGetInputPhotoDirect(t *testing.T) {
	photo := &tg.Photo{ID: 444, AccessHash: 555, FileReference: []byte("r")}
	ip, err := utils.GetInputPhoto(photo)
	if err != nil {
		t.Fatal(err)
	}
	if ip.ID != 444 || ip.AccessHash != 555 {
		t.Errorf("got ID=%d Hash=%d, want 444/555", ip.ID, ip.AccessHash)
	}
}

func TestGetInputPhotoFromMedia(t *testing.T) {
	photo := &tg.Photo{ID: 666}
	media := &tg.MessageMediaPhoto{Photo: photo}
	ip, err := utils.GetInputPhoto(media)
	if err != nil {
		t.Fatal(err)
	}
	if ip.ID != 666 {
		t.Errorf("got ID=%d, want 666", ip.ID)
	}
}

// ---------------------------------------------------------------------------
// GetAppropriatedPartSize
// ---------------------------------------------------------------------------

func TestGetAppropriatedPartSize(t *testing.T) {
	cases := []struct {
		size int64
		want int
	}{
		{0, 128},
		{104857600, 128},    // exactly 100 MB
		{104857601, 256},    // just over 100 MB
		{786432000, 256},    // exactly 750 MB
		{786432001, 512},    // just over 750 MB
		{1073741824, 512},   // 1 GB
	}
	for _, c := range cases {
		got := utils.GetAppropriatedPartSize(c.size)
		if got != c.want {
			t.Errorf("GetAppropriatedPartSize(%d) = %d, want %d", c.size, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ConcatBytes
// ---------------------------------------------------------------------------

func TestConcatBytes(t *testing.T) {
	a := []byte{1, 2, 3}
	b := []byte{4, 5}
	c := []byte{6}
	got := utils.ConcatBytes(a, b, c)
	want := []byte{1, 2, 3, 4, 5, 6}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("ConcatBytes[%d] = %d, want %d", i, got[i], v)
		}
	}
}

func TestConcatBytesEmpty(t *testing.T) {
	got := utils.ConcatBytes()
	if len(got) != 0 {
		t.Errorf("ConcatBytes() = %v, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// SanitizeParseMode
// ---------------------------------------------------------------------------

func TestSanitizeParseMode(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"md", "markdown"},
		{"Markdown", "markdown"},
		{"MARKDOWN", "markdown"},
		{"html", "html"},
		{"HTML", "html"},
		{"htm", "html"},
		{"unknown", "unknown"},
		{"", ""},
	}
	for _, c := range cases {
		got := utils.SanitizeParseMode(c.in)
		if got != c.want {
			t.Errorf("SanitizeParseMode(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------------
// TryGetInputPeer
// ---------------------------------------------------------------------------

func TestTryGetInputPeerValid(t *testing.T) {
	user := &tg.User{ID: 100, AccessHash: 42}
	got := utils.TryGetInputPeer(user)
	if got == nil {
		t.Fatal("TryGetInputPeer returned nil for valid user")
	}
	ipu, ok := got.(*tg.InputPeerUser)
	if !ok {
		t.Fatalf("got %T, want *tg.InputPeerUser", got)
	}
	if ipu.UserID != 100 {
		t.Errorf("UserID: got %d, want 100", ipu.UserID)
	}
}

func TestTryGetInputPeerInvalid(t *testing.T) {
	got := utils.TryGetInputPeer("not-an-entity")
	if got != nil {
		t.Errorf("expected nil for unknown type, got %T", got)
	}
}

// ---------------------------------------------------------------------------
// GetDocumentAttributes
// ---------------------------------------------------------------------------

func TestGetDocumentAttributes(t *testing.T) {
	attrs := []tg.DocumentAttributeClass{
		&tg.DocumentAttributeFilename{FileName: "file.mp4"},
		&tg.DocumentAttributeVideo{},
	}
	doc := &tg.Document{Attributes: attrs}
	got := utils.GetDocumentAttributes(doc)
	if len(got) != 2 {
		t.Errorf("expected 2 attributes, got %d", len(got))
	}
}

func TestGetDocumentAttributesNil(t *testing.T) {
	got := utils.GetDocumentAttributes(nil)
	if got != nil {
		t.Errorf("expected nil for nil document, got %v", got)
	}
}

// ---------------------------------------------------------------------------
// GetTotalBytes
// ---------------------------------------------------------------------------

func TestGetTotalBytesDocument(t *testing.T) {
	doc := &tg.Document{Size: 1024}
	media := &tg.MessageMediaDocument{Document: doc}
	got := utils.GetTotalBytes(media)
	if got != 1024 {
		t.Errorf("GetTotalBytes = %d, want 1024", got)
	}
}

func TestGetTotalBytesPhoto(t *testing.T) {
	media := &tg.MessageMediaPhoto{}
	got := utils.GetTotalBytes(media)
	if got != 0 {
		t.Errorf("GetTotalBytes for photo = %d, want 0", got)
	}
}

// ---------------------------------------------------------------------------
// GetInputMedia — smoke tests
// ---------------------------------------------------------------------------

func TestGetInputMediaPhoto(t *testing.T) {
	photo := &tg.Photo{ID: 10, AccessHash: 20, FileReference: []byte("r")}
	im, err := utils.GetInputMedia(photo, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := im.(*tg.InputMediaPhoto); !ok {
		t.Errorf("expected *tg.InputMediaPhoto, got %T", im)
	}
}

func TestGetInputMediaDocument(t *testing.T) {
	doc := &tg.Document{ID: 30, AccessHash: 40, FileReference: []byte("r")}
	im, err := utils.GetInputMedia(doc, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := im.(*tg.InputMediaDocument); !ok {
		t.Errorf("expected *tg.InputMediaDocument, got %T", im)
	}
}

func TestGetInputMediaContact(t *testing.T) {
	media := &tg.MessageMediaContact{
		PhoneNumber: "+1234567890",
		FirstName:   "Test",
		LastName:    "User",
	}
	im, err := utils.GetInputMedia(media, false)
	if err != nil {
		t.Fatal(err)
	}
	ic, ok := im.(*tg.InputMediaContact)
	if !ok {
		t.Fatalf("expected *tg.InputMediaContact, got %T", im)
	}
	if ic.PhoneNumber != "+1234567890" {
		t.Errorf("PhoneNumber: got %q, want %q", ic.PhoneNumber, "+1234567890")
	}
}

func TestGetInputMediaDice(t *testing.T) {
	media := &tg.MessageMediaDice{Emoticon: "🎲"}
	im, err := utils.GetInputMedia(media, false)
	if err != nil {
		t.Fatal(err)
	}
	id, ok := im.(*tg.InputMediaDice)
	if !ok {
		t.Fatalf("expected *tg.InputMediaDice, got %T", im)
	}
	if id.Emoticon != "🎲" {
		t.Errorf("Emoticon: got %q, want 🎲", id.Emoticon)
	}
}

func TestGetInputMediaAlreadyInput(t *testing.T) {
	original := &tg.InputMediaEmpty{}
	im, err := utils.GetInputMedia(original, false)
	if err != nil {
		t.Fatal(err)
	}
	if im != original {
		t.Errorf("passthrough failed: got %T, want the original *tg.InputMediaEmpty", im)
	}
}

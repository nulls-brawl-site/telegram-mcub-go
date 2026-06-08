package utils_test

import (
	"testing"

	"github.com/gotd/td/tg"
	"github.com/nulls-brawl-site/telegram-mcub-go/utils"
)

// ─────────────────────────────────────────────────────────────────────────────
// GetDisplayName
// ─────────────────────────────────────────────────────────────────────────────

func TestGetDisplayNameFullName(t *testing.T) {
	user := &tg.User{FirstName: "John", LastName: "Doe"}
	if got := utils.GetDisplayName(user); got != "John Doe" {
		t.Errorf("got %q, want %q", got, "John Doe")
	}
}

func TestGetDisplayNameFirstOnly(t *testing.T) {
	user := &tg.User{FirstName: "Alice"}
	if got := utils.GetDisplayName(user); got != "Alice" {
		t.Errorf("got %q, want %q", got, "Alice")
	}
}

func TestGetDisplayNameLastOnly(t *testing.T) {
	user := &tg.User{LastName: "Smith"}
	if got := utils.GetDisplayName(user); got != "Smith" {
		t.Errorf("got %q, want %q", got, "Smith")
	}
}

func TestGetDisplayNameEmpty(t *testing.T) {
	user := &tg.User{}
	if got := utils.GetDisplayName(user); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestGetDisplayNameChannel(t *testing.T) {
	ch := &tg.Channel{Title: "My Channel"}
	if got := utils.GetDisplayName(ch); got != "My Channel" {
		t.Errorf("got %q, want %q", got, "My Channel")
	}
}

func TestGetDisplayNameChat(t *testing.T) {
	chat := &tg.Chat{Title: "Group Chat"}
	if got := utils.GetDisplayName(chat); got != "Group Chat" {
		t.Errorf("got %q, want %q", got, "Group Chat")
	}
}

func TestGetDisplayNameChatForbidden(t *testing.T) {
	chat := &tg.ChatForbidden{Title: "Forbidden"}
	if got := utils.GetDisplayName(chat); got != "Forbidden" {
		t.Errorf("got %q, want %q", got, "Forbidden")
	}
}

func TestGetDisplayNameChannelForbidden(t *testing.T) {
	ch := &tg.ChannelForbidden{Title: "Forbidden Channel"}
	if got := utils.GetDisplayName(ch); got != "Forbidden Channel" {
		t.Errorf("got %q, want %q", got, "Forbidden Channel")
	}
}

func TestGetDisplayNameUnknownType(t *testing.T) {
	if got := utils.GetDisplayName("not a telegram entity"); got != "" {
		t.Errorf("got %q, want empty for unknown type", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetPeerID
// ─────────────────────────────────────────────────────────────────────────────

func TestGetPeerIDUser(t *testing.T) {
	peer := &tg.PeerUser{UserID: 123}
	got := utils.GetPeerID(peer, false)
	if got != 123 {
		t.Errorf("PeerUser: got %d, want 123", got)
	}
}

func TestGetPeerIDChat(t *testing.T) {
	// PeerChat returns raw ChatID (positive), no mark applied.
	peer := &tg.PeerChat{ChatID: 456}
	got := utils.GetPeerID(peer, false)
	if got != 456 {
		t.Errorf("PeerChat: got %d, want 456", got)
	}
}

func TestGetPeerIDChannelNoMark(t *testing.T) {
	peer := &tg.PeerChannel{ChannelID: 789}
	got := utils.GetPeerID(peer, false)
	if got != 789 {
		t.Errorf("PeerChannel (no mark): got %d, want 789", got)
	}
}

func TestGetPeerIDChannelWithMark(t *testing.T) {
	peer := &tg.PeerChannel{ChannelID: 789}
	got := utils.GetPeerID(peer, true)
	want := int64(-1_000_000_000_000 - 789)
	if got != want {
		t.Errorf("PeerChannel (mark): got %d, want %d", got, want)
	}
}

func TestGetPeerIDUserEntity(t *testing.T) {
	user := &tg.User{ID: 111}
	got := utils.GetPeerID(user, false)
	if got != 111 {
		t.Errorf("User: got %d, want 111", got)
	}
}

func TestGetPeerIDChatEntity(t *testing.T) {
	chat := &tg.Chat{ID: 222}
	got := utils.GetPeerID(chat, false)
	if got != 222 {
		t.Errorf("Chat: got %d, want 222", got)
	}
}

func TestGetPeerIDChannelEntityWithMark(t *testing.T) {
	ch := &tg.Channel{ID: 333}
	got := utils.GetPeerID(ch, true)
	want := int64(-1_000_000_000_000 - 333)
	if got != want {
		t.Errorf("Channel (mark): got %d, want %d", got, want)
	}
}

func TestGetPeerIDUnknownType(t *testing.T) {
	got := utils.GetPeerID("unknown", false)
	if got != 0 {
		t.Errorf("unknown type: got %d, want 0", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// MimeToExt / ExtToMime
// ─────────────────────────────────────────────────────────────────────────────

func TestMimeToExtKnown(t *testing.T) {
	cases := []struct {
		mime string
		ext  string
	}{
		{"image/jpeg", ".jpeg"},
		{"image/png", ".png"},
		{"video/mp4", ".mp4"},
		{"audio/mpeg", ".mp3"},
		{"audio/ogg", ".ogg"},
		{"image/gif", ".gif"},
		{"application/octet-stream", ""},
	}
	for _, c := range cases {
		got := utils.MimeToExt(c.mime)
		if got != c.ext {
			t.Errorf("MimeToExt(%q): got %q, want %q", c.mime, got, c.ext)
		}
	}
}

func TestMimeToExtUnknown(t *testing.T) {
	// Unknown MIME type should return empty string or a stdlib fallback.
	// We only assert it doesn't panic.
	_ = utils.MimeToExt("application/x-unknown-nonexistent-type")
}

// ─────────────────────────────────────────────────────────────────────────────
// GetExtension
// ─────────────────────────────────────────────────────────────────────────────

func TestGetExtensionPhoto(t *testing.T) {
	photo := &tg.Photo{}
	got := utils.GetExtension(photo)
	if got != ".jpg" {
		t.Errorf("Photo: got %q, want %q", got, ".jpg")
	}
}

func TestGetExtensionDocument(t *testing.T) {
	doc := &tg.Document{MimeType: "video/mp4"}
	got := utils.GetExtension(doc)
	if got != ".mp4" {
		t.Errorf("Document mp4: got %q, want %q", got, ".mp4")
	}
}

func TestGetExtensionDocumentMP3(t *testing.T) {
	doc := &tg.Document{MimeType: "audio/mpeg"}
	got := utils.GetExtension(doc)
	if got != ".mp3" {
		t.Errorf("Document mp3: got %q, want %q", got, ".mp3")
	}
}

func TestGetExtensionMessageMediaDocument(t *testing.T) {
	media := &tg.MessageMediaDocument{
		Document: &tg.Document{MimeType: "image/png"},
	}
	got := utils.GetExtension(media)
	if got != ".png" {
		t.Errorf("MessageMediaDocument png: got %q, want %q", got, ".png")
	}
}

func TestGetExtensionUserProfilePhoto(t *testing.T) {
	photo := &tg.UserProfilePhoto{}
	got := utils.GetExtension(photo)
	if got != ".jpg" {
		t.Errorf("UserProfilePhoto: got %q, want %q", got, ".jpg")
	}
}

func TestGetExtensionUnknown(t *testing.T) {
	got := utils.GetExtension("not a media type")
	if got != "" {
		t.Errorf("unknown: got %q, want empty", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ValidUsername / ResolveUsername
// ─────────────────────────────────────────────────────────────────────────────

func TestValidUsername(t *testing.T) {
	// Telegram usernames: start with letter, 5-32 chars, only letters/digits/underscores,
	// no double underscores, end with letter/digit.
	valid := []string{
		"telegram",
		"test_bot",
		"abc12",  // 5 chars — minimum valid length with this regex
	}
	for _, u := range valid {
		if !utils.ValidUsername(u) {
			t.Errorf("ValidUsername(%q) = false, want true", u)
		}
	}
}

func TestInvalidUsername(t *testing.T) {
	invalid := []string{
		"",
		"a",           // too short (only 1 char)
		"ab",          // too short (only 2 chars, needs at least 3)
		"__starts",    // starts with underscore
		"has space",   // contains space
		"has__double", // contains double underscore
	}
	for _, u := range invalid {
		if utils.ValidUsername(u) {
			t.Errorf("ValidUsername(%q) = true, want false", u)
		}
	}
}

func TestResolveUsername(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"@telegram", "telegram"},
		{"https://t.me/telegram", "telegram"},
		{"t.me/telegram", "telegram"},
	}
	for _, c := range cases {
		got := utils.ResolveUsername(c.input)
		if got != c.want {
			t.Errorf("ResolveUsername(%q): got %q, want %q", c.input, got, c.want)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Chunks
// ─────────────────────────────────────────────────────────────────────────────

func TestChunksEven(t *testing.T) {
	chunks := utils.Chunks([]int{1, 2, 3, 4}, 2)
	if len(chunks) != 2 {
		t.Fatalf("chunk count: got %d, want 2", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 {
		t.Errorf("chunk sizes: got %v, want [2 2]", []int{len(chunks[0]), len(chunks[1])})
	}
}

func TestChunksUneven(t *testing.T) {
	chunks := utils.Chunks([]int{1, 2, 3, 4, 5}, 2)
	if len(chunks) != 3 {
		t.Fatalf("chunk count: got %d, want 3", len(chunks))
	}
	if len(chunks[2]) != 1 {
		t.Errorf("last chunk size: got %d, want 1", len(chunks[2]))
	}
}

func TestChunksSingle(t *testing.T) {
	chunks := utils.Chunks([]int{1, 2, 3}, 10)
	if len(chunks) != 1 {
		t.Fatalf("chunk count: got %d, want 1", len(chunks))
	}
	if len(chunks[0]) != 3 {
		t.Errorf("chunk size: got %d, want 3", len(chunks[0]))
	}
}

func TestChunksEmpty(t *testing.T) {
	chunks := utils.Chunks([]int{}, 5)
	if len(chunks) != 0 {
		t.Errorf("chunk count: got %d, want 0", len(chunks))
	}
}

func TestChunksZeroSize(t *testing.T) {
	chunks := utils.Chunks([]int{1, 2, 3}, 0)
	if chunks != nil {
		t.Errorf("zero size: got %v, want nil", chunks)
	}
}

func TestChunksStrings(t *testing.T) {
	chunks := utils.Chunks([]string{"a", "b", "c", "d"}, 3)
	if len(chunks) != 2 {
		t.Fatalf("chunk count: got %d, want 2", len(chunks))
	}
	if len(chunks[0]) != 3 || len(chunks[1]) != 1 {
		t.Errorf("chunk sizes: got [%d,%d], want [3,1]", len(chunks[0]), len(chunks[1]))
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetPeer
// ─────────────────────────────────────────────────────────────────────────────

func TestGetPeerUserID(t *testing.T) {
	peer := utils.GetPeer(123)
	pu, ok := peer.(*tg.PeerUser)
	if !ok {
		t.Fatalf("got %T, want *tg.PeerUser", peer)
	}
	if pu.UserID != 123 {
		t.Errorf("UserID: got %d, want 123", pu.UserID)
	}
}

func TestGetPeerChannelID(t *testing.T) {
	// Negative ID → channel.
	peer := utils.GetPeer(-1_000_000_000_789)
	pc, ok := peer.(*tg.PeerChannel)
	if !ok {
		t.Fatalf("got %T, want *tg.PeerChannel", peer)
	}
	if pc.ChannelID != 789 {
		t.Errorf("ChannelID: got %d, want 789", pc.ChannelID)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetFileName / GetFileSize / GetMimeType
// ─────────────────────────────────────────────────────────────────────────────

func TestGetFileName(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeFilename{FileName: "test.mp4"},
		},
	}
	got := utils.GetFileName(doc)
	if got != "test.mp4" {
		t.Errorf("got %q, want %q", got, "test.mp4")
	}
}

func TestGetFileNameMissing(t *testing.T) {
	doc := &tg.Document{}
	if got := utils.GetFileName(doc); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestGetFileSize(t *testing.T) {
	doc := &tg.Document{Size: 1024}
	got := utils.GetFileSize(doc)
	if got != 1024 {
		t.Errorf("got %d, want 1024", got)
	}
}

func TestGetMimeTypeDocument(t *testing.T) {
	doc := &tg.Document{MimeType: "video/mp4"}
	if got := utils.GetMimeType(doc); got != "video/mp4" {
		t.Errorf("got %q, want %q", got, "video/mp4")
	}
}

func TestGetMimeTypePhoto(t *testing.T) {
	media := &tg.MessageMediaPhoto{}
	if got := utils.GetMimeType(media); got != "image/jpeg" {
		t.Errorf("got %q, want %q", got, "image/jpeg")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// IsImage / IsVideo / IsAudio etc.
// ─────────────────────────────────────────────────────────────────────────────

func TestIsImage(t *testing.T) {
	if !utils.IsImage(&tg.Photo{}) {
		t.Error("Photo should be an image")
	}
	if !utils.IsImage(&tg.MessageMediaPhoto{}) {
		t.Error("MessageMediaPhoto should be an image")
	}
	if utils.IsImage(&tg.Document{}) {
		t.Error("Document should not be an image")
	}
}

func TestIsVideo(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeVideo{},
		},
	}
	if !utils.IsVideo(doc) {
		t.Error("expected IsVideo=true")
	}
	if utils.IsVideo(&tg.Document{}) {
		t.Error("empty document should not be a video")
	}
}

func TestIsAudio(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAudio{},
		},
	}
	if !utils.IsAudio(doc) {
		t.Error("expected IsAudio=true")
	}
}

func TestIsSticker(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeSticker{},
		},
	}
	if !utils.IsSticker(doc) {
		t.Error("expected IsSticker=true")
	}
}

func TestIsGIF(t *testing.T) {
	doc := &tg.Document{
		Attributes: []tg.DocumentAttributeClass{
			&tg.DocumentAttributeAnimated{},
		},
	}
	if !utils.IsGIF(doc) {
		t.Error("expected IsGIF=true")
	}
}

func TestIsChannel(t *testing.T) {
	if !utils.IsChannel(&tg.Channel{}) {
		t.Error("Channel should be a channel")
	}
	if !utils.IsChannel(&tg.PeerChannel{}) {
		t.Error("PeerChannel should be a channel")
	}
	if utils.IsChannel(&tg.User{}) {
		t.Error("User should not be a channel")
	}
}

func TestIsMegagroup(t *testing.T) {
	ch := &tg.Channel{Megagroup: true}
	if !utils.IsMegagroup(ch) {
		t.Error("expected IsMegagroup=true")
	}
	ch2 := &tg.Channel{Megagroup: false}
	if utils.IsMegagroup(ch2) {
		t.Error("expected IsMegagroup=false")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// FileExtension
// ─────────────────────────────────────────────────────────────────────────────

func TestFileExtension(t *testing.T) {
	cases := []struct {
		name string
		ext  string
	}{
		{"video.MP4", ".mp4"},
		{"photo.JPG", ".jpg"},
		{"document.PDF", ".pdf"},
		{"no_ext", ""},
	}
	for _, c := range cases {
		got := utils.FileExtension(c.name)
		if got != c.ext {
			t.Errorf("FileExtension(%q): got %q, want %q", c.name, got, c.ext)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// GetInputPeer / GetInputChannel / GetInputUser
// ─────────────────────────────────────────────────────────────────────────────

func TestGetInputPeerUser(t *testing.T) {
	user := &tg.User{ID: 100, AccessHash: 999}
	ip, err := utils.GetInputPeer(user, false)
	if err != nil {
		t.Fatal(err)
	}
	ipu, ok := ip.(*tg.InputPeerUser)
	if !ok {
		t.Fatalf("got %T, want *tg.InputPeerUser", ip)
	}
	if ipu.UserID != 100 || ipu.AccessHash != 999 {
		t.Errorf("UserID/AccessHash: got %d/%d, want 100/999", ipu.UserID, ipu.AccessHash)
	}
}

func TestGetInputPeerChannel(t *testing.T) {
	ch := &tg.Channel{ID: 200, AccessHash: 888}
	ip, err := utils.GetInputPeer(ch, false)
	if err != nil {
		t.Fatal(err)
	}
	ipc, ok := ip.(*tg.InputPeerChannel)
	if !ok {
		t.Fatalf("got %T, want *tg.InputPeerChannel", ip)
	}
	if ipc.ChannelID != 200 || ipc.AccessHash != 888 {
		t.Errorf("ChannelID/AccessHash: got %d/%d, want 200/888", ipc.ChannelID, ipc.AccessHash)
	}
}

func TestGetInputPeerUnknown(t *testing.T) {
	_, err := utils.GetInputPeer("unknown", false)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestGetInputChannelEntity(t *testing.T) {
	ch := &tg.Channel{ID: 300, AccessHash: 777}
	ic, err := utils.GetInputChannel(ch)
	if err != nil {
		t.Fatal(err)
	}
	icc, ok := ic.(*tg.InputChannel)
	if !ok {
		t.Fatalf("got %T, want *tg.InputChannel", ic)
	}
	if icc.ChannelID != 300 {
		t.Errorf("ChannelID: got %d, want 300", icc.ChannelID)
	}
}

func TestGetInputUserEntity(t *testing.T) {
	user := &tg.User{ID: 400, AccessHash: 666}
	iu, err := utils.GetInputUser(user)
	if err != nil {
		t.Fatal(err)
	}
	iuu, ok := iu.(*tg.InputUser)
	if !ok {
		t.Fatalf("got %T, want *tg.InputUser", iu)
	}
	if iuu.UserID != 400 {
		t.Errorf("UserID: got %d, want 400", iuu.UserID)
	}
}

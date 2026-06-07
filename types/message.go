package types

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"
)

// ParseMode controls how message text is parsed.
type ParseMode int

const (
	// ParseModeNone sends the text as-is with no formatting.
	ParseModeNone ParseMode = iota
	// ParseModeHTML interprets the text as HTML (Telegram's HTML subset + MCUB extensions).
	ParseModeHTML
	// ParseModeMarkdown interprets the text as Markdown V2.
	ParseModeMarkdown
)

// SendMessageOptions contains options for sending a message.
type SendMessageOptions struct {
	// ParseMode controls text formatting.
	ParseMode ParseMode

	// Buttons is the optional keyboard markup.
	Buttons ButtonGrid

	// ReplyToMsgID is the ID of the message to reply to.
	ReplyToMsgID int

	// Silent suppresses notifications for the recipient.
	Silent bool

	// ScheduleDate schedules the message for future delivery (Unix timestamp).
	ScheduleDate int

	// ClearDraft clears the chat draft after sending.
	ClearDraft bool

	// NoWebpage disables link preview.
	NoWebpage bool

	// ForumTopicID is the topic ID when sending to a forum supergroup topic.
	ForumTopicID int

	// ProtectContent prevents forwarding/saving.
	ProtectContent bool
}

// SendFileOptions contains options for sending a file.
type SendFileOptions struct {
	// Caption is the optional file caption.
	Caption string

	// ParseMode controls caption formatting.
	ParseMode ParseMode

	// Buttons is the optional keyboard markup.
	Buttons ButtonGrid

	// ReplyToMsgID is the ID of the message to reply to.
	ReplyToMsgID int

	// Silent suppresses notifications.
	Silent bool

	// ForumTopicID is the topic ID when sending to a forum topic.
	ForumTopicID int

	// ProtectContent prevents forwarding/saving.
	ProtectContent bool

	// FileName overrides the upload file name.
	FileName string

	// MimeType overrides the detected MIME type.
	MimeType string

	// AsDocument forces the file to be sent as a document (not media).
	AsDocument bool

	// Thumb is an optional thumbnail for video/audio files.
	Thumb string

	// Duration is the duration in seconds (for audio/video).
	Duration int

	// Width/Height for images or videos.
	Width  int
	Height int
}

// Message is an extended Telegram message with MCUB helper fields.
type Message struct {
	// Raw is the underlying gotd/td message.
	Raw *tg.Message

	// PeerID is the resolved peer ID.
	PeerID int64

	// FromID is the sender's user ID (0 for channel posts).
	FromID int64

	// Date is the parsed message timestamp.
	Date time.Time

	// Text is the plain-text message body.
	Text string

	// Entities are the parsed formatting entities.
	Entities []tg.MessageEntityClass

	// IsForumTopic indicates the message belongs to a forum topic.
	IsForumTopic bool

	// ForumTopicID is the topic ID (only set when IsForumTopic is true).
	ForumTopicID int

	// ReplyToMsgID is the replied-to message ID.
	ReplyToMsgID int
}

// ---------------------------------------------------------------------------
// MCUBMessage — rich wrapper ported from Telethon's tl/custom/message.py
// ---------------------------------------------------------------------------

// MCUBMessage wraps *tg.Message with rich helper methods that mirror the
// Telethon custom Message class.
type MCUBMessage struct {
	// Raw is the underlying gotd/td message.
	Raw *tg.Message

	// Client is a back-reference to the owning client; typed as interface{}
	// to avoid an import cycle.  Callers may type-assert to *client.MCUBClient.
	Client interface{}

	// ChatID is the signed peer ID of the conversation.
	ChatID int64

	// SenderID is the user ID of the sender (0 for anonymous posts).
	SenderID int64
}

// ID returns the message ID.
func (m *MCUBMessage) ID() int {
	if m.Raw == nil {
		return 0
	}
	return m.Raw.ID
}

// Text returns the plain-text message body.
func (m *MCUBMessage) Text() string {
	if m.Raw == nil {
		return ""
	}
	return m.Raw.Message
}

// Date returns the UTC timestamp of the message.
func (m *MCUBMessage) Date() time.Time {
	if m.Raw == nil {
		return time.Time{}
	}
	return time.Unix(int64(m.Raw.Date), 0)
}

// IsOutgoing reports whether the message was sent by the local user.
func (m *MCUBMessage) IsOutgoing() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.Out
}

// IsReply reports whether the message is a reply to another message.
func (m *MCUBMessage) IsReply() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.ReplyTo != nil
}

// ReplyToID returns the ID of the message being replied to (0 if not a reply).
func (m *MCUBMessage) ReplyToID() int {
	if m.Raw == nil || m.Raw.ReplyTo == nil {
		return 0
	}
	if rt, ok := m.Raw.ReplyTo.(*tg.MessageReplyHeader); ok {
		return rt.ReplyToMsgID
	}
	return 0
}

// HasMedia reports whether the message carries any media.
func (m *MCUBMessage) HasMedia() bool {
	return m.Raw != nil && m.Raw.Media != nil
}

// IsPhoto reports whether the message media is a photo.
func (m *MCUBMessage) IsPhoto() bool {
	if !m.HasMedia() {
		return false
	}
	_, ok := m.Raw.Media.(*tg.MessageMediaPhoto)
	return ok
}

// IsDocument reports whether the message media is a document.
func (m *MCUBMessage) IsDocument() bool {
	if !m.HasMedia() {
		return false
	}
	_, ok := m.Raw.Media.(*tg.MessageMediaDocument)
	return ok
}

// document returns the tg.Document if the media is a document, else nil.
func (m *MCUBMessage) document() *tg.Document {
	if !m.IsDocument() {
		return nil
	}
	mmd, ok := m.Raw.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil
	}
	doc, ok := mmd.Document.(*tg.Document)
	if !ok {
		return nil
	}
	return doc
}

// hasDocumentAttr checks whether the document has an attribute of the given type.
func (m *MCUBMessage) hasDocumentAttr(check func(tg.DocumentAttributeClass) bool) bool {
	doc := m.document()
	if doc == nil {
		return false
	}
	for _, attr := range doc.Attributes {
		if check(attr) {
			return true
		}
	}
	return false
}

// IsVideo reports whether the message is a video.
func (m *MCUBMessage) IsVideo() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		if va, ok := a.(*tg.DocumentAttributeVideo); ok {
			return !va.RoundMessage
		}
		return false
	})
}

// IsAudio reports whether the message is an audio file (not a voice note).
func (m *MCUBMessage) IsAudio() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			return !aa.Voice
		}
		return false
	})
}

// IsVoice reports whether the message is a voice note.
func (m *MCUBMessage) IsVoice() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			return aa.Voice
		}
		return false
	})
}

// IsSticker reports whether the message is a sticker.
func (m *MCUBMessage) IsSticker() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		_, ok := a.(*tg.DocumentAttributeSticker)
		return ok
	})
}

// IsGIF reports whether the message is an animated GIF/mp4.
func (m *MCUBMessage) IsGIF() bool {
	return m.hasDocumentAttr(func(a tg.DocumentAttributeClass) bool {
		_, ok := a.(*tg.DocumentAttributeAnimated)
		return ok
	})
}

// FileName returns the file name of the document, if any.
func (m *MCUBMessage) FileName() string {
	doc := m.document()
	if doc == nil {
		return ""
	}
	for _, attr := range doc.Attributes {
		if fa, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return fa.FileName
		}
	}
	return ""
}

// FileSize returns the file size in bytes (0 if no document).
func (m *MCUBMessage) FileSize() int64 {
	doc := m.document()
	if doc == nil {
		return 0
	}
	return doc.Size
}

// MimeType returns the MIME type of the document, or an empty string.
func (m *MCUBMessage) MimeType() string {
	doc := m.document()
	if doc == nil {
		return ""
	}
	return doc.MimeType
}

// Caption returns the message text when it accompanies media (same as Text).
func (m *MCUBMessage) Caption() string {
	return m.Text()
}

// IsForwarded reports whether the message was forwarded from another chat.
func (m *MCUBMessage) IsForwarded() bool {
	if m.Raw == nil {
		return false
	}
	_, ok := m.Raw.GetFwdFrom()
	return ok
}

// ForwardedFromID returns the sender ID of the original message, or 0.
func (m *MCUBMessage) ForwardedFromID() int64 {
	if m.Raw == nil {
		return 0
	}
	fwd, ok := m.Raw.GetFwdFrom()
	if !ok {
		return 0
	}
	fromID, hasFrom := fwd.GetFromID()
	if !hasFrom {
		return 0
	}
	switch p := fromID.(type) {
	case *tg.PeerUser:
		return int64(p.UserID)
	case *tg.PeerChannel:
		return int64(p.ChannelID)
	}
	return 0
}

// Via returns the username of the inline bot the message was sent via, or "".
func (m *MCUBMessage) Via() string {
	if m.Raw == nil {
		return ""
	}
	// ViaBotsID is the bot's user ID; without a name resolver we return the
	// numeric ID as a string representation stub — callers can resolve it.
	_, ok := m.Raw.GetViaBotID()
	if !ok {
		return ""
	}
	return "" // Name resolution requires the client; return blank here.
}

// GroupedID returns the album group ID (non-zero when message belongs to an album).
func (m *MCUBMessage) GroupedID() int64 {
	if m.Raw == nil {
		return 0
	}
	id, ok := m.Raw.GetGroupedID()
	if !ok {
		return 0
	}
	return id
}

// Buttons parses the reply markup and returns the grid of MessageButtons.
// Returns nil when there are no buttons.
func (m *MCUBMessage) Buttons() [][]*MessageButton {
	if m.Raw == nil || m.Raw.ReplyMarkup == nil {
		return nil
	}
	return ParseMarkup(m.Raw.ReplyMarkup)
}

// GetButton returns the button at the given (row, col) position, or nil.
func (m *MCUBMessage) GetButton(row, col int) *MessageButton {
	grid := m.Buttons()
	if row < 0 || row >= len(grid) {
		return nil
	}
	r := grid[row]
	if col < 0 || col >= len(r) {
		return nil
	}
	return r[col]
}

// ---------------------------------------------------------------------------
// Additional accessors mirroring Telethon's custom Message class
// ---------------------------------------------------------------------------

// RawText returns the plain message text without any markdown formatting.
// Equivalent to Telethon's raw_text property.
func (m *MCUBMessage) RawText() string {
	if m.Raw == nil {
		return ""
	}
	return m.Raw.Message
}

// IsEdited reports whether the message has been edited.
func (m *MCUBMessage) IsEdited() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.EditDate != 0
}

// IsPinned reports whether the message is currently pinned.
func (m *MCUBMessage) IsPinned() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.Pinned
}

// IsSilent reports whether the message was sent silently.
func (m *MCUBMessage) IsSilent() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.Silent
}

// IsScheduled reports whether the message originated from a scheduled message.
func (m *MCUBMessage) IsScheduled() bool {
	if m.Raw == nil {
		return false
	}
	return m.Raw.FromScheduled
}

// IsService reports whether this is a service message (always false for tg.Message).
// Service messages use tg.MessageService; this wrapper only wraps tg.Message.
func (m *MCUBMessage) IsService() bool {
	return false
}

// ReplyToMsgID returns the ID of the message being replied to (0 if not a reply).
// Alias for ReplyToID() for Telethon naming compatibility.
func (m *MCUBMessage) ReplyToMsgID() int {
	return m.ReplyToID()
}

// ReplyToTopID returns the thread/topic ID for forum topics, or 0.
func (m *MCUBMessage) ReplyToTopID() int {
	if m.Raw == nil || m.Raw.ReplyTo == nil {
		return 0
	}
	if rt, ok := m.Raw.ReplyTo.(*tg.MessageReplyHeader); ok {
		id, _ := rt.GetReplyToTopID()
		return id
	}
	return 0
}

// ForwardedFrom returns an MCUBForward wrapping the forward header, or nil.
func (m *MCUBMessage) ForwardedFrom() *MCUBForward {
	if m.Raw == nil {
		return nil
	}
	fwd, ok := m.Raw.GetFwdFrom()
	if !ok {
		return nil
	}
	return NewForward(&fwd)
}

// Entities returns the message formatting entities.
func (m *MCUBMessage) Entities() []tg.MessageEntityClass {
	if m.Raw == nil {
		return nil
	}
	return m.Raw.Entities
}

// File returns an MCUBFile wrapping the photo or document in this message.
// Returns nil when the media type is not a file (polls, dice, etc.).
func (m *MCUBMessage) File() *MCUBFile {
	if m.Raw == nil {
		return nil
	}
	return NewFile(m.Raw)
}

// Photo returns the *tg.Photo from the message media, or nil.
func (m *MCUBMessage) Photo() *tg.Photo {
	if !m.HasMedia() {
		return nil
	}
	mmp, ok := m.Raw.Media.(*tg.MessageMediaPhoto)
	if !ok {
		return nil
	}
	photo, ok := mmp.Photo.(*tg.Photo)
	if !ok {
		return nil
	}
	return photo
}

// Document returns the *tg.Document from the message media, or nil.
func (m *MCUBMessage) Document() *tg.Document {
	return m.document()
}

// Audio returns the document if it's an audio track (not a voice note), else nil.
func (m *MCUBMessage) Audio() *tg.Document {
	return m.documentByAttr(func(a tg.DocumentAttributeClass) bool {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			return !aa.Voice
		}
		return false
	})
}

// Voice returns the document if it's a voice note, else nil.
func (m *MCUBMessage) Voice() *tg.Document {
	return m.documentByAttr(func(a tg.DocumentAttributeClass) bool {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			return aa.Voice
		}
		return false
	})
}

// Video returns the document if it's a video (not a video note), else nil.
func (m *MCUBMessage) Video() *tg.Document {
	return m.documentByAttr(func(a tg.DocumentAttributeClass) bool {
		if va, ok := a.(*tg.DocumentAttributeVideo); ok {
			return !va.RoundMessage
		}
		return false
	})
}

// VideoNote returns the document if it's a video note (round message), else nil.
func (m *MCUBMessage) VideoNote() *tg.Document {
	return m.documentByAttr(func(a tg.DocumentAttributeClass) bool {
		if va, ok := a.(*tg.DocumentAttributeVideo); ok {
			return va.RoundMessage
		}
		return false
	})
}

// Sticker returns the document if it's a sticker, else nil.
func (m *MCUBMessage) Sticker() *tg.Document {
	return m.documentByAttr(func(a tg.DocumentAttributeClass) bool {
		_, ok := a.(*tg.DocumentAttributeSticker)
		return ok
	})
}

// GIF returns the document if it's an animated GIF/mp4, else nil.
func (m *MCUBMessage) GIF() *tg.Document {
	return m.documentByAttr(func(a tg.DocumentAttributeClass) bool {
		_, ok := a.(*tg.DocumentAttributeAnimated)
		return ok
	})
}

// documentByAttr returns the document if it has an attribute matching the predicate.
func (m *MCUBMessage) documentByAttr(pred func(tg.DocumentAttributeClass) bool) *tg.Document {
	doc := m.document()
	if doc == nil {
		return nil
	}
	for _, attr := range doc.Attributes {
		if pred(attr) {
			return doc
		}
	}
	return nil
}

// Contact returns the contact media, or nil.
func (m *MCUBMessage) Contact() *tg.MessageMediaContact {
	if !m.HasMedia() {
		return nil
	}
	c, _ := m.Raw.Media.(*tg.MessageMediaContact)
	return c
}

// Location returns the geo media, or nil.
func (m *MCUBMessage) Location() *tg.MessageMediaGeo {
	if !m.HasMedia() {
		return nil
	}
	g, _ := m.Raw.Media.(*tg.MessageMediaGeo)
	return g
}

// Venue returns the venue media, or nil.
func (m *MCUBMessage) Venue() *tg.MessageMediaVenue {
	if !m.HasMedia() {
		return nil
	}
	v, _ := m.Raw.Media.(*tg.MessageMediaVenue)
	return v
}

// Poll returns the poll media, or nil.
func (m *MCUBMessage) Poll() *tg.MessageMediaPoll {
	if !m.HasMedia() {
		return nil
	}
	p, _ := m.Raw.Media.(*tg.MessageMediaPoll)
	return p
}

// Dice returns the dice media, or nil.
func (m *MCUBMessage) Dice() *tg.MessageMediaDice {
	if !m.HasMedia() {
		return nil
	}
	d, _ := m.Raw.Media.(*tg.MessageMediaDice)
	return d
}

// Game returns the game media, or nil.
func (m *MCUBMessage) Game() *tg.MessageMediaGame {
	if !m.HasMedia() {
		return nil
	}
	g, _ := m.Raw.Media.(*tg.MessageMediaGame)
	return g
}

// WebPreview returns the web page media, or nil.
func (m *MCUBMessage) WebPreview() *tg.MessageMediaWebPage {
	if !m.HasMedia() {
		return nil
	}
	w, _ := m.Raw.Media.(*tg.MessageMediaWebPage)
	return w
}

// GetButtonByText returns the first button whose text matches, or nil.
func (m *MCUBMessage) GetButtonByText(text string) *MessageButton {
	for _, row := range m.Buttons() {
		for _, btn := range row {
			if btn.Text == text {
				return btn
			}
		}
	}
	return nil
}

// GetButtonByData returns the first button whose data equals data, or nil.
func (m *MCUBMessage) GetButtonByData(data []byte) *MessageButton {
	for _, row := range m.Buttons() {
		for _, btn := range row {
			if bytes.Equal(btn.Data, data) {
				return btn
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Action methods (require a client that implements the relevant interface)
// ---------------------------------------------------------------------------

// Reply sends a message as a reply to this message and returns the sent message.
func (m *MCUBMessage) Reply(ctx context.Context, text string) (*MCUBMessage, error) {
	if m.Raw == nil {
		return nil, fmt.Errorf("nil message")
	}
	type replyClient interface {
		SendReply(ctx context.Context, chatID int64, text string, replyToMsgID int) (*tg.Message, error)
	}
	c, ok := m.Client.(replyClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement SendReply")
	}
	sent, err := c.SendReply(ctx, m.ChatID, text, m.Raw.ID)
	if err != nil {
		return nil, err
	}
	return NewMCUBMessage(sent, m.Client), nil
}

// Respond sends a message to the same chat without replying to a specific message.
func (m *MCUBMessage) Respond(ctx context.Context, text string) (*MCUBMessage, error) {
	if m.Raw == nil {
		return nil, fmt.Errorf("nil message")
	}
	type respondClient interface {
		SendText(ctx context.Context, chatID int64, text string) (*tg.Message, error)
	}
	c, ok := m.Client.(respondClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement SendText")
	}
	sent, err := c.SendText(ctx, m.ChatID, text)
	if err != nil {
		return nil, err
	}
	return NewMCUBMessage(sent, m.Client), nil
}

// Edit edits the message text.
func (m *MCUBMessage) Edit(ctx context.Context, text string) (*MCUBMessage, error) {
	if m.Raw == nil {
		return nil, fmt.Errorf("nil message")
	}
	type editClient interface {
		EditMessage(ctx context.Context, chatID int64, msgID int, text string) (*tg.Message, error)
	}
	c, ok := m.Client.(editClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement EditMessage")
	}
	edited, err := c.EditMessage(ctx, m.ChatID, m.Raw.ID, text)
	if err != nil {
		return nil, err
	}
	return NewMCUBMessage(edited, m.Client), nil
}

// Delete deletes the message.  If revoke is true, it is also deleted for the other party.
func (m *MCUBMessage) Delete(ctx context.Context, revoke bool) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	type deleteClient interface {
		DeleteMessages(ctx context.Context, chatID int64, msgIDs []int, revoke bool) error
	}
	c, ok := m.Client.(deleteClient)
	if !ok {
		return fmt.Errorf("client does not implement DeleteMessages")
	}
	return c.DeleteMessages(ctx, m.ChatID, []int{m.Raw.ID}, revoke)
}

// Pin pins the message.  If notify is true, members receive a notification.
func (m *MCUBMessage) Pin(ctx context.Context, notify bool) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	type pinClient interface {
		PinMessage(ctx context.Context, chatID int64, msgID int, notify bool) error
	}
	c, ok := m.Client.(pinClient)
	if !ok {
		return fmt.Errorf("client does not implement PinMessage")
	}
	return c.PinMessage(ctx, m.ChatID, m.Raw.ID, notify)
}

// Unpin unpins the message.
func (m *MCUBMessage) Unpin(ctx context.Context) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	type unpinClient interface {
		UnpinMessage(ctx context.Context, chatID int64, msgID int) error
	}
	c, ok := m.Client.(unpinClient)
	if !ok {
		return fmt.Errorf("client does not implement UnpinMessage")
	}
	return c.UnpinMessage(ctx, m.ChatID, m.Raw.ID)
}

// Forward forwards this message to another chat and returns the forwarded copy.
func (m *MCUBMessage) Forward(ctx context.Context, toChatID int64) (*MCUBMessage, error) {
	if m.Raw == nil {
		return nil, fmt.Errorf("nil message")
	}
	type forwardClient interface {
		ForwardMessages(ctx context.Context, fromChatID int64, msgIDs []int, toChatID int64) ([]*tg.Message, error)
	}
	c, ok := m.Client.(forwardClient)
	if !ok {
		return nil, fmt.Errorf("client does not implement ForwardMessages")
	}
	msgs, err := c.ForwardMessages(ctx, m.ChatID, []int{m.Raw.ID}, toChatID)
	if err != nil || len(msgs) == 0 {
		return nil, err
	}
	return NewMCUBMessage(msgs[0], m.Client), nil
}

// Download downloads the message media to filePath.
func (m *MCUBMessage) Download(ctx context.Context, filePath string) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	type downloadClient interface {
		DownloadMedia(ctx context.Context, msg *tg.Message, filePath string) error
	}
	c, ok := m.Client.(downloadClient)
	if !ok {
		return fmt.Errorf("client does not implement DownloadMedia")
	}
	return c.DownloadMedia(ctx, m.Raw, filePath)
}

// GetReplyMessage fetches the message this one is replying to, or nil.
func (m *MCUBMessage) GetReplyMessage(ctx context.Context) (*MCUBMessage, error) {
	if m.Raw == nil || m.Raw.ReplyTo == nil {
		return nil, nil
	}
	rt, ok := m.Raw.ReplyTo.(*tg.MessageReplyHeader)
	if !ok {
		return nil, nil
	}
	type getMsgClient interface {
		GetMessage(ctx context.Context, chatID int64, msgID int) (*tg.Message, error)
	}
	c, ok2 := m.Client.(getMsgClient)
	if !ok2 {
		return nil, fmt.Errorf("client does not implement GetMessage")
	}
	raw, err := c.GetMessage(ctx, m.ChatID, rt.ReplyToMsgID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return NewMCUBMessage(raw, m.Client), nil
}

// ClickButton clicks the button at position (row, col) in the inline keyboard.
func (m *MCUBMessage) ClickButton(ctx context.Context, row, col int) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	btn := m.GetButton(row, col)
	if btn == nil {
		return fmt.Errorf("button not found at (%d, %d)", row, col)
	}
	type clickClient interface {
		ClickButton(ctx context.Context, chatID int64, msgID int, data []byte) error
	}
	c, ok := m.Client.(clickClient)
	if !ok {
		return fmt.Errorf("client does not implement ClickButton")
	}
	return c.ClickButton(ctx, m.ChatID, m.Raw.ID, btn.Data)
}

// React sends an emoji reaction to this message.
func (m *MCUBMessage) React(ctx context.Context, emoji string) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	type reactClient interface {
		SendReaction(ctx context.Context, chatID int64, msgID int, emoji string) error
	}
	c, ok := m.Client.(reactClient)
	if !ok {
		return fmt.Errorf("client does not implement SendReaction")
	}
	return c.SendReaction(ctx, m.ChatID, m.Raw.ID, emoji)
}

// MarkRead marks this message and all previous ones in the chat as read.
func (m *MCUBMessage) MarkRead(ctx context.Context) error {
	if m.Raw == nil {
		return fmt.Errorf("nil message")
	}
	type markReadClient interface {
		MarkRead(ctx context.Context, chatID int64) error
	}
	c, ok := m.Client.(markReadClient)
	if !ok {
		return fmt.Errorf("client does not implement MarkRead")
	}
	return c.MarkRead(ctx, m.ChatID)
}

// NewMessage constructs an MCUBMessage with an explicit chatID override.
// Use this when you already know the chat ID (e.g. from an update container).
func NewMessage(raw *tg.Message, client interface{}, chatID int64) *MCUBMessage {
	m := NewMCUBMessage(raw, client)
	if chatID != 0 {
		m.ChatID = chatID
	}
	return m
}

// NewMCUBMessage wraps a raw tg.Message into a MCUBMessage, resolving peer IDs.
func NewMCUBMessage(raw *tg.Message, client interface{}) *MCUBMessage {
	m := &MCUBMessage{Raw: raw, Client: client}
	if raw == nil {
		return m
	}
	if raw.PeerID != nil {
		switch p := raw.PeerID.(type) {
		case *tg.PeerUser:
			m.ChatID = int64(p.UserID)
		case *tg.PeerChat:
			m.ChatID = -int64(p.ChatID)
		case *tg.PeerChannel:
			m.ChatID = -1000000000000 - int64(p.ChannelID)
		}
	}
	if raw.FromID != nil {
		switch f := raw.FromID.(type) {
		case *tg.PeerUser:
			m.SenderID = int64(f.UserID)
		case *tg.PeerChannel:
			m.SenderID = -1000000000000 - int64(f.ChannelID)
		}
	}
	return m
}

// ---------------------------------------------------------------------------

// FromTLMessage converts a tg.Message to the extended Message type.
func FromTLMessage(m *tg.Message) *Message {
	msg := &Message{
		Raw:      m,
		Text:     m.Message,
		Entities: m.Entities,
		Date:     time.Unix(int64(m.Date), 0),
	}

	// Resolve peer ID.
	if m.PeerID != nil {
		switch p := m.PeerID.(type) {
		case *tg.PeerUser:
			msg.PeerID = int64(p.UserID)
		case *tg.PeerChat:
			msg.PeerID = -int64(p.ChatID)
		case *tg.PeerChannel:
			msg.PeerID = -1000000000000 - int64(p.ChannelID)
		}
	}

	// Resolve sender.
	if m.FromID != nil {
		switch f := m.FromID.(type) {
		case *tg.PeerUser:
			msg.FromID = int64(f.UserID)
		case *tg.PeerChannel:
			msg.FromID = int64(f.ChannelID)
		}
	}

	// Forum topic fields.
	if m.ReplyTo != nil {
		if rt, ok := m.ReplyTo.(*tg.MessageReplyHeader); ok {
			if rt.ForumTopic {
				msg.IsForumTopic = true
				msg.ForumTopicID, _ = rt.GetReplyToTopID()
			}
			msg.ReplyToMsgID = rt.ReplyToMsgID
		}
	}

	return msg
}

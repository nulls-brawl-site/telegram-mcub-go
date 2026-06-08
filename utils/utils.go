// Package utils provides utility functions that mirror key parts of
// telethon/utils.py and telethon/helpers.py.
package utils

import (
	"mime"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gotd/td/tg"
)

// ---------------------------------------------------------------------------
// Display name helpers (get_display_name)
// ---------------------------------------------------------------------------

// GetDisplayName returns the human-readable display name for a Telegram
// entity (User, Chat, or Channel).  Returns an empty string for unknown types.
func GetDisplayName(entity interface{}) string {
	switch e := entity.(type) {
	case *tg.User:
		switch {
		case e.FirstName != "" && e.LastName != "":
			return e.FirstName + " " + e.LastName
		case e.FirstName != "":
			return e.FirstName
		case e.LastName != "":
			return e.LastName
		default:
			return ""
		}
	case *tg.Chat:
		return e.Title
	case *tg.ChatForbidden:
		return e.Title
	case *tg.Channel:
		return e.Title
	case *tg.ChannelForbidden:
		return e.Title
	}
	return ""
}

// ---------------------------------------------------------------------------
// File extension / MIME-type helpers
// ---------------------------------------------------------------------------

// Common MIME-type → extension mappings (mirrors Telethon's mimetypes.add_type calls).
var mimeToExtMap = map[string]string{
	"image/png":                   ".png",
	"image/jpeg":                  ".jpeg",
	"image/webp":                  ".webp",
	"image/gif":                   ".gif",
	"image/bmp":                   ".bmp",
	"image/x-tga":                 ".tga",
	"image/tiff":                  ".tiff",
	"image/vnd.adobe.photoshop":   ".psd",
	"video/mp4":                   ".mp4",
	"video/quicktime":             ".mov",
	"video/avi":                   ".avi",
	"audio/mpeg":                  ".mp3",
	"audio/m4a":                   ".m4a",
	"audio/aac":                   ".aac",
	"audio/ogg":                   ".ogg",
	"audio/flac":                  ".flac",
	"application/x-tgsticker":     ".tgs",
	"application/octet-stream":    "",
}

// extToMimeMap is the reverse of mimeToExtMap.
var extToMimeMap map[string]string

func init() {
	extToMimeMap = make(map[string]string, len(mimeToExtMap))
	for mtype, ext := range mimeToExtMap {
		if ext != "" {
			extToMimeMap[ext] = mtype
		}
	}
}

// MimeToExt returns the file extension for a MIME type (e.g. ".jpg").
// Returns an empty string if the MIME type is unknown.
func MimeToExt(mimeType string) string {
	if ext, ok := mimeToExtMap[mimeType]; ok {
		return ext
	}
	if mimeType == "application/octet-stream" {
		return ""
	}
	// Fall back to the standard library.
	exts, err := mime.ExtensionsByType(mimeType)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

// ExtToMime returns the MIME type for a file extension (e.g. ".mp4").
func ExtToMime(ext string) string {
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	ext = strings.ToLower(ext)
	if mtype, ok := extToMimeMap[ext]; ok {
		return mtype
	}
	return mime.TypeByExtension(ext)
}

// GetExtension returns the file extension for Telegram media.
func GetExtension(media interface{}) string {
	switch m := media.(type) {
	case *tg.Photo, *tg.InputPhoto:
		return ".jpg"
	case *tg.UserProfilePhoto, *tg.ChatPhoto:
		return ".jpg"
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return MimeToExt(doc.MimeType)
		}
	case *tg.Document:
		return MimeToExt(m.MimeType)
	case *tg.WebDocument:
		return MimeToExt(m.MimeType)
	case *tg.WebDocumentNoProxy:
		return MimeToExt(m.MimeType)
	}
	return ""
}

// GetMimeType returns the MIME type of Telegram media.
func GetMimeType(media interface{}) string {
	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return doc.MimeType
		}
	case *tg.Document:
		return m.MimeType
	case *tg.WebDocument:
		return m.MimeType
	case *tg.WebDocumentNoProxy:
		return m.MimeType
	case *tg.MessageMediaPhoto:
		return "image/jpeg"
	}
	return ""
}

// GetFileName returns the filename from document attributes.
func GetFileName(media interface{}) string {
	var attrs []tg.DocumentAttributeClass
	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			attrs = doc.Attributes
		}
	case *tg.Document:
		attrs = m.Attributes
	}
	for _, attr := range attrs {
		if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
			return fn.FileName
		}
	}
	return ""
}

// GetFileSize returns the size in bytes of media.
func GetFileSize(media interface{}) int64 {
	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return doc.Size
		}
	case *tg.Document:
		return m.Size
	case *tg.WebDocument:
		return int64(m.Size)
	case *tg.WebDocumentNoProxy:
		return int64(m.Size)
	}
	return 0
}

// ---------------------------------------------------------------------------
// Media type predicates
// ---------------------------------------------------------------------------

// IsImage returns true if media is a photo.
func IsImage(media interface{}) bool {
	switch media.(type) {
	case *tg.Photo, *tg.MessageMediaPhoto, *tg.InputPhoto:
		return true
	}
	return false
}

// IsVideo returns true if the document is a video.
func IsVideo(media interface{}) bool {
	attrs := documentAttrs(media)
	for _, a := range attrs {
		if _, ok := a.(*tg.DocumentAttributeVideo); ok {
			return true
		}
	}
	return false
}

// IsAudio returns true if the document is audio.
func IsAudio(media interface{}) bool {
	attrs := documentAttrs(media)
	for _, a := range attrs {
		if _, ok := a.(*tg.DocumentAttributeAudio); ok {
			return true
		}
	}
	return false
}

// IsVoice returns true if the document is a voice message.
func IsVoice(media interface{}) bool {
	attrs := documentAttrs(media)
	for _, a := range attrs {
		if audio, ok := a.(*tg.DocumentAttributeAudio); ok {
			return audio.Voice
		}
	}
	return false
}

// IsSticker returns true if the document is a sticker.
func IsSticker(media interface{}) bool {
	attrs := documentAttrs(media)
	for _, a := range attrs {
		if _, ok := a.(*tg.DocumentAttributeSticker); ok {
			return true
		}
	}
	return false
}

// IsGIF returns true if the document is an animated GIF.
func IsGIF(media interface{}) bool {
	attrs := documentAttrs(media)
	for _, a := range attrs {
		if _, ok := a.(*tg.DocumentAttributeAnimated); ok {
			return true
		}
	}
	return false
}

// documentAttrs extracts the attribute slice from a document-like media value.
func documentAttrs(media interface{}) []tg.DocumentAttributeClass {
	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return doc.Attributes
		}
	case *tg.Document:
		return m.Attributes
	}
	return nil
}

// ---------------------------------------------------------------------------
// Peer ID helpers (port of get_peer_id / get_peer)
// ---------------------------------------------------------------------------

// GetPeerID returns the canonical numeric peer ID.
// If addMark is true, channels and supergroups get a negative mark
// (multiplied by -1_000_000_000_000) to distinguish them from users/chats.
func GetPeerID(peer interface{}, addMark bool) int64 {
	switch p := peer.(type) {
	case *tg.PeerUser:
		return p.UserID
	case *tg.PeerChat:
		return p.ChatID
	case *tg.PeerChannel:
		if addMark {
			return -1_000_000_000_000 - p.ChannelID
		}
		return p.ChannelID
	case *tg.User:
		return p.ID
	case *tg.Chat, *tg.ChatForbidden:
		switch c := p.(type) {
		case *tg.Chat:
			return c.ID
		case *tg.ChatForbidden:
			return c.ID
		}
	case *tg.Channel:
		if addMark {
			return -1_000_000_000_000 - p.ID
		}
		return p.ID
	case *tg.ChannelForbidden:
		if addMark {
			return -1_000_000_000_000 - p.ID
		}
		return p.ID
	}
	return 0
}

// GetPeer converts a numeric peer ID to the appropriate PeerClass.
func GetPeer(peerID int64) tg.PeerClass {
	if peerID < 0 {
		return &tg.PeerChannel{ChannelID: -(peerID + 1_000_000_000_000)}
	}
	return &tg.PeerUser{UserID: peerID}
}

// ---------------------------------------------------------------------------
// Input peer/channel/user helpers
// ---------------------------------------------------------------------------

// GetInputPeer returns an InputPeerClass for the given entity.
func GetInputPeer(entity interface{}, allowSelf bool) (tg.InputPeerClass, error) {
	switch e := entity.(type) {
	case *tg.InputPeerSelf:
		return e, nil
	case *tg.InputPeerUser:
		return e, nil
	case *tg.InputPeerChat:
		return e, nil
	case *tg.InputPeerChannel:
		return e, nil
	case *tg.InputPeerEmpty:
		return e, nil
	case *tg.User:
		if e.Self && allowSelf {
			return &tg.InputPeerSelf{}, nil
		}
		return &tg.InputPeerUser{UserID: e.ID, AccessHash: e.AccessHash}, nil
	case *tg.Chat:
		return &tg.InputPeerChat{ChatID: e.ID}, nil
	case *tg.ChatForbidden:
		return &tg.InputPeerChat{ChatID: e.ID}, nil
	case *tg.Channel:
		return &tg.InputPeerChannel{ChannelID: e.ID, AccessHash: e.AccessHash}, nil
	case *tg.ChannelForbidden:
		return &tg.InputPeerChannel{ChannelID: e.ID, AccessHash: e.AccessHash}, nil
	}
	return nil, &InputError{entity}
}

// GetInputChannel returns an InputChannelClass for the given entity.
func GetInputChannel(entity interface{}) (tg.InputChannelClass, error) {
	switch e := entity.(type) {
	case *tg.InputChannel:
		return e, nil
	case *tg.InputChannelEmpty:
		return e, nil
	case *tg.Channel:
		return &tg.InputChannel{ChannelID: e.ID, AccessHash: e.AccessHash}, nil
	case *tg.ChannelForbidden:
		return &tg.InputChannel{ChannelID: e.ID, AccessHash: e.AccessHash}, nil
	case *tg.InputPeerChannel:
		return &tg.InputChannel{ChannelID: e.ChannelID, AccessHash: e.AccessHash}, nil
	}
	return nil, &InputError{entity}
}

// GetInputUser returns an InputUserClass for the given entity.
func GetInputUser(entity interface{}) (tg.InputUserClass, error) {
	switch e := entity.(type) {
	case *tg.InputUser:
		return e, nil
	case *tg.InputUserSelf:
		return e, nil
	case *tg.InputUserEmpty:
		return e, nil
	case *tg.User:
		if e.Self {
			return &tg.InputUserSelf{}, nil
		}
		return &tg.InputUser{UserID: e.ID, AccessHash: e.AccessHash}, nil
	case *tg.InputPeerUser:
		return &tg.InputUser{UserID: e.UserID, AccessHash: e.AccessHash}, nil
	}
	return nil, &InputError{entity}
}

// InputError is returned when an entity cannot be converted to an input type.
type InputError struct{ Entity interface{} }

func (e *InputError) Error() string {
	return "cannot convert entity to input type"
}

// ---------------------------------------------------------------------------
// Channel / megagroup predicates
// ---------------------------------------------------------------------------

// IsChannel returns true if the entity is a channel or supergroup.
func IsChannel(entity interface{}) bool {
	switch entity.(type) {
	case *tg.Channel, *tg.ChannelForbidden,
		*tg.InputPeerChannel, *tg.InputChannel,
		*tg.PeerChannel:
		return true
	}
	return false
}

// IsMegagroup returns true if the entity is a supergroup (megagroup).
func IsMegagroup(entity interface{}) bool {
	if ch, ok := entity.(*tg.Channel); ok {
		return ch.Megagroup
	}
	return false
}

// ---------------------------------------------------------------------------
// Username helpers
// ---------------------------------------------------------------------------

var (
	usernameRE      = regexp.MustCompile(`@|(?:https?://)?(?:www\.)?(?:telegram\.(?:me|dog)|t\.me)/(@|\+|joinchat/)?`)
	// RE2 does not support negative lookaheads, so we use a two-step check:
	// 1. basic structure: starts with letter, 1-30 word chars, ends with letter/digit
	// 2. does not contain "__"  (enforced in ValidUsername)
	validUsernameRE = regexp.MustCompile(`(?i)^[a-z]\w{1,30}[a-z\d]$`)
)

// ValidUsername reports whether s is a valid Telegram username (without @).
// Mirrors Telethon's VALID_USERNAME_RE but uses a two-step check because
// Go's RE2 engine does not support negative lookaheads.
func ValidUsername(username string) bool {
	if strings.Contains(username, "__") {
		return false
	}
	return validUsernameRE.MatchString(username)
}

// ResolveUsername strips @, t.me/ and similar prefixes from a username string.
func ResolveUsername(username string) string {
	return usernameRE.ReplaceAllString(username, "")
}

// ---------------------------------------------------------------------------
// Chunk helper
// ---------------------------------------------------------------------------

// Chunks splits items into sub-slices of at most size elements.
func Chunks[T any](items []T, size int) [][]T {
	if size <= 0 {
		return nil
	}
	var chunks [][]T
	for len(items) > 0 {
		end := size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[:end])
		items = items[end:]
	}
	return chunks
}

// ---------------------------------------------------------------------------
// Message ID helper
// ---------------------------------------------------------------------------

// GetMessageID extracts the message ID from a Message or related type.
func GetMessageID(msg interface{}) int {
	switch m := msg.(type) {
	case *tg.Message:
		return m.ID
	case *tg.MessageService:
		return m.ID
	case *tg.MessageEmpty:
		return m.ID
	}
	return 0
}

// ---------------------------------------------------------------------------
// Misc path/extension helpers
// ---------------------------------------------------------------------------

// FileExtension returns the lowercased extension of a filename (e.g. ".mp4").
func FileExtension(filename string) string {
	return strings.ToLower(filepath.Ext(filename))
}

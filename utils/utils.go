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
// Byte-slice helpers
// ---------------------------------------------------------------------------

// ConcatBytes concatenates multiple byte slices into a single slice.
func ConcatBytes(slices ...[]byte) []byte {
	total := 0
	for _, s := range slices {
		total += len(s)
	}
	out := make([]byte, 0, total)
	for _, s := range slices {
		out = append(out, s...)
	}
	return out
}

// ---------------------------------------------------------------------------
// Parse helpers (port of parse_phone / parse_username from utils.py)
// ---------------------------------------------------------------------------

// ParsePhone normalises a phone number string by stripping +, (, ), spaces and
// dashes.  Returns the digits-only string, or "" if the result contains
// non-digit characters.  Mirrors Telethon's parse_phone().
func ParsePhone(phone string) string {
	re := regexp.MustCompile(`[+()\s\-]`)
	cleaned := re.ReplaceAllString(phone, "")
	for _, ch := range cleaned {
		if ch < '0' || ch > '9' {
			return ""
		}
	}
	return cleaned
}

var tgJoinRE = regexp.MustCompile(`tg://(join)\?invite=`)

// ParseUsername parses a username or invite-link string and returns
// (username, isInvite).
//
//   - "@user"            → ("user", false)
//   - "https://t.me/user" → ("user", false)
//   - "t.me/+hash"       → ("hash", true)   (joinchat / invite hash)
//
// Returns ("", false) when the input cannot be recognised as a valid username
// or invite link.
//
// Mirrors Telethon's parse_username() from telethon/utils.py.
func ParseUsername(s string) (string, bool) {
	s = strings.TrimSpace(s)

	// Try the regular t.me / telegram.me URL pattern.
	loc := usernameRE.FindStringIndex(s)
	// Also try the tg://join?invite= pattern.
	if loc == nil {
		loc2 := tgJoinRE.FindStringSubmatchIndex(s)
		if loc2 != nil {
			rest := s[loc2[1]:]
			return rest, true
		}
	}
	if loc != nil {
		// Determine whether the matched prefix captured a +/joinchat/ group.
		sub := usernameRE.FindStringSubmatch(s)
		isInvite := len(sub) > 1 && (sub[1] == "+" || sub[1] == "joinchat/")
		rest := s[loc[1]:]
		if isInvite {
			return rest, true
		}
		rest = strings.TrimRight(rest, "/")
		s = rest
	}

	if ValidUsername(s) {
		return strings.ToLower(s), false
	}
	return "", false
}

// ---------------------------------------------------------------------------
// Upload / download helpers
// ---------------------------------------------------------------------------

// GetAppropriatedPartSize returns the recommended upload/download chunk size
// in kilobytes for the given file size.  Mirrors Telethon's
// get_appropriated_part_size().
//
//	≤ 100 MB → 128 KB
//	≤ 750 MB → 256 KB
//	otherwise → 512 KB
func GetAppropriatedPartSize(fileSize int64) int {
	if fileSize <= 104857600 { // 100 MB
		return 128
	}
	if fileSize <= 786432000 { // 750 MB
		return 256
	}
	return 512
}

// ---------------------------------------------------------------------------
// Document attribute helpers
// ---------------------------------------------------------------------------

// IsRound returns true if the document is a round video note.
// Mirrors Telethon's is_video() combined with the round_message flag.
func IsRound(media interface{}) bool {
	for _, a := range documentAttrs(media) {
		if v, ok := a.(*tg.DocumentAttributeVideo); ok {
			return v.RoundMessage
		}
	}
	return false
}

// IsAnimated returns true if the document is an animated TGS sticker.
func IsAnimated(media interface{}) bool {
	switch m := media.(type) {
	case *tg.Document:
		return m.MimeType == "application/x-tgsticker"
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return doc.MimeType == "application/x-tgsticker"
		}
	}
	return false
}

// GetDocumentAttributes returns the full attribute list of a Document.
func GetDocumentAttributes(doc *tg.Document) []tg.DocumentAttributeClass {
	if doc == nil {
		return nil
	}
	return doc.Attributes
}

// ---------------------------------------------------------------------------
// Input type conversion helpers
// ---------------------------------------------------------------------------

// GetInputDocument converts a Document (or Document-containing media) to an
// InputDocument.  Mirrors Telethon's get_input_document().
func GetInputDocument(doc interface{}) (*tg.InputDocument, error) {
	switch d := doc.(type) {
	case *tg.InputDocument:
		return d, nil
	case *tg.Document:
		return &tg.InputDocument{
			ID:            d.ID,
			AccessHash:    d.AccessHash,
			FileReference: d.FileReference,
		}, nil
	case *tg.MessageMediaDocument:
		if inner, ok := d.Document.(*tg.Document); ok {
			return GetInputDocument(inner)
		}
	case *tg.Message:
		return GetInputDocument(d.Media)
	}
	return nil, &InputError{doc}
}

// GetInputPhoto converts a Photo (or Photo-containing media) to an InputPhoto.
// Mirrors Telethon's get_input_photo().
func GetInputPhoto(photo interface{}) (*tg.InputPhoto, error) {
	switch p := photo.(type) {
	case *tg.InputPhoto:
		return p, nil
	case *tg.Photo:
		return &tg.InputPhoto{
			ID:            p.ID,
			AccessHash:    p.AccessHash,
			FileReference: p.FileReference,
		}, nil
	case *tg.MessageMediaPhoto:
		if inner, ok := p.Photo.(*tg.Photo); ok {
			return GetInputPhoto(inner)
		}
	case *tg.Message:
		return GetInputPhoto(p.Media)
	}
	return nil, &InputError{photo}
}

// GetInputChatPhoto converts a photo value to an InputChatPhotoClass.
// Mirrors Telethon's get_input_chat_photo().
func GetInputChatPhoto(photo interface{}) (tg.InputChatPhotoClass, error) {
	switch p := photo.(type) {
	case *tg.InputChatPhoto:
		return p, nil
	case *tg.InputChatPhotoEmpty:
		return p, nil
	case *tg.InputChatUploadedPhoto:
		return p, nil
	}
	inputPhoto, err := GetInputPhoto(photo)
	if err != nil {
		return nil, err
	}
	return &tg.InputChatPhoto{ID: inputPhoto}, nil
}

// GetInputMedia converts a media value to the appropriate InputMediaClass.
// Set isPhoto=true when the upload should be treated as a photo.
// Mirrors Telethon's get_input_media() from telethon/utils.py.
func GetInputMedia(media interface{}, isPhoto bool) (tg.InputMediaClass, error) {
	// Already an InputMedia — pass through.
	if im, ok := media.(tg.InputMediaClass); ok {
		return im, nil
	}

	switch m := media.(type) {
	case *tg.Photo:
		ip, err := GetInputPhoto(m)
		if err != nil {
			return nil, err
		}
		return &tg.InputMediaPhoto{ID: ip}, nil

	case *tg.MessageMediaPhoto:
		inner, ok := m.Photo.(*tg.Photo)
		if !ok {
			return &tg.InputMediaEmpty{}, nil
		}
		ip, err := GetInputPhoto(inner)
		if err != nil {
			return nil, err
		}
		return &tg.InputMediaPhoto{ID: ip}, nil

	case *tg.Document:
		id, err := GetInputDocument(m)
		if err != nil {
			return nil, err
		}
		return &tg.InputMediaDocument{ID: id}, nil

	case *tg.MessageMediaDocument:
		id, err := GetInputDocument(m.Document)
		if err != nil {
			return nil, err
		}
		return &tg.InputMediaDocument{ID: id}, nil

	case *tg.InputFile:
		if isPhoto {
			return &tg.InputMediaUploadedPhoto{File: m}, nil
		}
		attrs, mime := GetAttributes(m.Name, false, false, false, false)
		return &tg.InputMediaUploadedDocument{
			File:       m,
			MimeType:   mime,
			Attributes: attrs,
		}, nil

	case *tg.InputFileBig:
		if isPhoto {
			return &tg.InputMediaUploadedPhoto{File: m}, nil
		}
		attrs, mime := GetAttributes(m.Name, false, false, false, false)
		return &tg.InputMediaUploadedDocument{
			File:       m,
			MimeType:   mime,
			Attributes: attrs,
		}, nil

	case *tg.MessageMediaGeo:
		geoPoint, ok := m.Geo.(*tg.GeoPoint)
		if !ok {
			return &tg.InputMediaEmpty{}, nil
		}
		return &tg.InputMediaGeoPoint{
			GeoPoint: &tg.InputGeoPoint{Lat: geoPoint.Lat, Long: geoPoint.Long},
		}, nil

	case *tg.MessageMediaGeoLive:
		geoPoint, ok := m.Geo.(*tg.GeoPoint)
		if !ok {
			return &tg.InputMediaEmpty{}, nil
		}
		return &tg.InputMediaGeoPoint{
			GeoPoint: &tg.InputGeoPoint{Lat: geoPoint.Lat, Long: geoPoint.Long},
		}, nil

	case *tg.MessageMediaContact:
		return &tg.InputMediaContact{
			PhoneNumber: m.PhoneNumber,
			FirstName:   m.FirstName,
			LastName:    m.LastName,
			Vcard:       "",
		}, nil

	case *tg.MessageMediaDice:
		return &tg.InputMediaDice{Emoticon: m.Emoticon}, nil

	case *tg.Message:
		return GetInputMedia(m.Media, isPhoto)

	case *tg.MessageMediaEmpty, *tg.MessageMediaUnsupported:
		return &tg.InputMediaEmpty{}, nil
	}

	return nil, &InputError{media}
}

// GetAttributes returns a list of document attributes and the MIME type for
// the given file path, honouring the caller's audio/voice/video/round flags.
//
// When the MIME type cannot be determined, "application/octet-stream" is
// returned.  Mirrors Telethon's get_attributes() from telethon/utils.py
// (without the optional hachoir metadata path).
func GetAttributes(filePath string, isAudio, isVoice, isVideo, isRound bool) ([]tg.DocumentAttributeClass, string) {
	ext := strings.ToLower(filepath.Ext(filePath))
	mimeType := ExtToMime(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	attrs := []tg.DocumentAttributeClass{
		&tg.DocumentAttributeFilename{FileName: filepath.Base(filePath)},
	}

	if isAudio || isVoice {
		attrs = append(attrs, &tg.DocumentAttributeAudio{Voice: isVoice})
	}
	if isVideo {
		attrs = append(attrs, &tg.DocumentAttributeVideo{RoundMessage: isRound})
	}

	return attrs, mimeType
}

// ---------------------------------------------------------------------------
// Message helpers
// ---------------------------------------------------------------------------

// GetTLMessage extracts a *tg.Message from various update types.
// Returns nil when the update does not carry a regular message.
// Mirrors Telethon's internal message-extraction helpers.
func GetTLMessage(u tg.UpdateClass) *tg.Message {
	switch upd := u.(type) {
	case *tg.UpdateNewMessage:
		if msg, ok := upd.Message.(*tg.Message); ok {
			return msg
		}
	case *tg.UpdateEditMessage:
		if msg, ok := upd.Message.(*tg.Message); ok {
			return msg
		}
	case *tg.UpdateNewChannelMessage:
		if msg, ok := upd.Message.(*tg.Message); ok {
			return msg
		}
	case *tg.UpdateEditChannelMessage:
		if msg, ok := upd.Message.(*tg.Message); ok {
			return msg
		}
	case *tg.UpdateNewScheduledMessage:
		if msg, ok := upd.Message.(*tg.Message); ok {
			return msg
		}
	}
	return nil
}

// GetMessageGroupID returns the grouped_id for an album message, or 0 if the
// message is not part of an album.
func GetMessageGroupID(msg *tg.Message) int64 {
	if msg == nil {
		return 0
	}
	if id, ok := msg.GetGroupedID(); ok {
		return id
	}
	return 0
}

// ---------------------------------------------------------------------------
// Parse mode helper
// ---------------------------------------------------------------------------

// SanitizeParseMode normalises a parse-mode string to one of the canonical
// values ("markdown", "html") or returns the input unchanged if unrecognised.
// Mirrors Telethon's sanitize_parse_mode() for the string-argument path.
func SanitizeParseMode(mode string) string {
	switch strings.ToLower(mode) {
	case "md", "markdown":
		return "markdown"
	case "htm", "html":
		return "html"
	}
	return mode
}

// ---------------------------------------------------------------------------
// Total bytes helper
// ---------------------------------------------------------------------------

// GetTotalBytes returns the total byte size of a MessageMedia value.
// Returns 0 for photos (size not reliably known without fetching) and for
// unsupported media types.
func GetTotalBytes(media tg.MessageMediaClass) int64 {
	switch m := media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			return doc.Size
		}
	case *tg.MessageMediaWebPage:
		if wp, ok := m.Webpage.(*tg.WebPage); ok {
			if doc, ok2 := wp.Document.(*tg.Document); ok2 {
				return doc.Size
			}
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// TryGetInputPeer
// ---------------------------------------------------------------------------

// TryGetInputPeer attempts to build an InputPeerClass from entity.
// Returns nil (instead of an error) when the conversion is not possible.
func TryGetInputPeer(entity interface{}) tg.InputPeerClass {
	ip, err := GetInputPeer(entity, true)
	if err != nil {
		return nil
	}
	return ip
}

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

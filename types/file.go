package types

import (
	"path/filepath"
	"strings"

	"github.com/gotd/td/tg"
)

// MCUBFile wraps file/media info from a Telegram message.
// It mirrors Telethon's tl/custom/file.File class.
type MCUBFile struct {
	// Raw is either a *tg.Document or *tg.Photo (or nil).
	Raw interface{}

	// Message is the source message containing the media.
	Message *tg.Message
}

// NewFile creates an MCUBFile from the media embedded in a tg.Message.
// Returns nil if the message has no supported media.
func NewFile(msg *tg.Message) *MCUBFile {
	if msg == nil || msg.Media == nil {
		return nil
	}
	f := &MCUBFile{Message: msg}
	switch m := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			f.Raw = doc
		}
	case *tg.MessageMediaPhoto:
		if photo, ok := m.Photo.(*tg.Photo); ok {
			f.Raw = photo
		}
	}
	if f.Raw == nil {
		return nil
	}
	return f
}

// document returns the underlying *tg.Document, or nil.
func (f *MCUBFile) document() *tg.Document {
	if f == nil {
		return nil
	}
	doc, _ := f.Raw.(*tg.Document)
	return doc
}

// photo returns the underlying *tg.Photo, or nil.
func (f *MCUBFile) photo() *tg.Photo {
	if f == nil {
		return nil
	}
	p, _ := f.Raw.(*tg.Photo)
	return p
}

// docAttr iterates document attributes and calls fn for each one, returning
// the first non-nil result.
func (f *MCUBFile) docAttr(fn func(tg.DocumentAttributeClass) interface{}) interface{} {
	doc := f.document()
	if doc == nil {
		return nil
	}
	for _, attr := range doc.Attributes {
		if v := fn(attr); v != nil {
			return v
		}
	}
	return nil
}

// Name returns the file name from DocumentAttributeFilename, or "" for photos.
func (f *MCUBFile) Name() string {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if fa, ok := a.(*tg.DocumentAttributeFilename); ok {
			return fa.FileName
		}
		return nil
	})
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Ext returns the file extension (e.g. ".jpg").
// Falls back to guessing from MimeType.
func (f *MCUBFile) Ext() string {
	name := f.Name()
	if name != "" {
		return filepath.Ext(name)
	}
	mime := f.MimeType()
	switch mime {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "video/mp4":
		return ".mp4"
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "application/pdf":
		return ".pdf"
	case "application/zip":
		return ".zip"
	}
	return ""
}

// MimeType returns the MIME type of the file.
func (f *MCUBFile) MimeType() string {
	if f.IsPhoto() {
		return "image/jpeg"
	}
	doc := f.document()
	if doc == nil {
		return ""
	}
	return doc.MimeType
}

// Size returns the file size in bytes (0 for photos without a known size).
func (f *MCUBFile) Size() int64 {
	doc := f.document()
	if doc != nil {
		return doc.Size
	}
	// For photos we can't easily get size without iterating sizes.
	return 0
}

// IsPhoto reports whether the media is a photo.
func (f *MCUBFile) IsPhoto() bool {
	return f.photo() != nil
}

// IsDocument reports whether the media is a document.
func (f *MCUBFile) IsDocument() bool {
	return f.document() != nil
}

// IsVideo reports whether the document is a video (not a video note).
func (f *MCUBFile) IsVideo() bool {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if va, ok := a.(*tg.DocumentAttributeVideo); ok && !va.RoundMessage {
			return true
		}
		return nil
	})
	return v != nil
}

// IsAudio reports whether the document is an audio track (not a voice note).
func (f *MCUBFile) IsAudio() bool {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok && !aa.Voice {
			return true
		}
		return nil
	})
	return v != nil
}

// IsVoice reports whether the document is a voice note.
func (f *MCUBFile) IsVoice() bool {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok && aa.Voice {
			return true
		}
		return nil
	})
	return v != nil
}

// IsSticker reports whether the document is a sticker.
func (f *MCUBFile) IsSticker() bool {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if _, ok := a.(*tg.DocumentAttributeSticker); ok {
			return true
		}
		return nil
	})
	return v != nil
}

// IsGIF reports whether the document is an animated GIF/mp4.
func (f *MCUBFile) IsGIF() bool {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if _, ok := a.(*tg.DocumentAttributeAnimated); ok {
			return true
		}
		return nil
	})
	return v != nil
}

// IsAnimated reports whether the file is an animated sticker or GIF.
func (f *MCUBFile) IsAnimated() bool {
	return f.IsGIF() || (f.IsSticker() && strings.HasSuffix(f.MimeType(), "tgsticker"))
}

// Duration returns the duration in seconds for audio or video documents.
func (f *MCUBFile) Duration() int {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		switch at := a.(type) {
		case *tg.DocumentAttributeAudio:
			return at.Duration
		case *tg.DocumentAttributeVideo:
			return at.Duration
		}
		return nil
	})
	if d, ok := v.(int); ok {
		return d
	}
	return 0
}

// Width returns the width in pixels for photo or video media.
func (f *MCUBFile) Width() int {
	if photo := f.photo(); photo != nil {
		maxW := 0
		for _, sz := range photo.Sizes {
			if ps, ok := sz.(*tg.PhotoSize); ok && ps.W > maxW {
				maxW = ps.W
			}
		}
		return maxW
	}
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		switch at := a.(type) {
		case *tg.DocumentAttributeImageSize:
			return at.W
		case *tg.DocumentAttributeVideo:
			return at.W
		}
		return nil
	})
	if w, ok := v.(int); ok {
		return w
	}
	return 0
}

// Height returns the height in pixels for photo or video media.
func (f *MCUBFile) Height() int {
	if photo := f.photo(); photo != nil {
		maxH := 0
		for _, sz := range photo.Sizes {
			if ps, ok := sz.(*tg.PhotoSize); ok && ps.H > maxH {
				maxH = ps.H
			}
		}
		return maxH
	}
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		switch at := a.(type) {
		case *tg.DocumentAttributeImageSize:
			return at.H
		case *tg.DocumentAttributeVideo:
			return at.H
		}
		return nil
	})
	if h, ok := v.(int); ok {
		return h
	}
	return 0
}

// Title returns the audio track title, if present.
func (f *MCUBFile) Title() string {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			t, _ := aa.GetTitle()
			return t
		}
		return nil
	})
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// Performer returns the audio track artist/performer, if present.
func (f *MCUBFile) Performer() string {
	v := f.docAttr(func(a tg.DocumentAttributeClass) interface{} {
		if aa, ok := a.(*tg.DocumentAttributeAudio); ok {
			p, _ := aa.GetPerformer()
			return p
		}
		return nil
	})
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// IsSupported reports whether the file type is a known/handled type.
func (f *MCUBFile) IsSupported() bool {
	return f.IsPhoto() || f.IsDocument()
}

// ID returns the document or photo ID (0 if not available).
func (f *MCUBFile) ID() int64 {
	if doc := f.document(); doc != nil {
		return doc.ID
	}
	if photo := f.photo(); photo != nil {
		return photo.ID
	}
	return 0
}

// AccessHash returns the document or photo access hash.
func (f *MCUBFile) AccessHash() int64 {
	if doc := f.document(); doc != nil {
		return doc.AccessHash
	}
	if photo := f.photo(); photo != nil {
		return photo.AccessHash
	}
	return 0
}

// FileReference returns the file reference bytes used for downloading.
func (f *MCUBFile) FileReference() []byte {
	if doc := f.document(); doc != nil {
		return doc.FileReference
	}
	if photo := f.photo(); photo != nil {
		return photo.FileReference
	}
	return nil
}

// Thumbs returns the thumbnail list for the document or photo.
func (f *MCUBFile) Thumbs() []interface{} {
	if doc := f.document(); doc != nil {
		out := make([]interface{}, len(doc.Thumbs))
		for i, t := range doc.Thumbs {
			out[i] = t
		}
		return out
	}
	if photo := f.photo(); photo != nil {
		out := make([]interface{}, len(photo.Sizes))
		for i, s := range photo.Sizes {
			out[i] = s
		}
		return out
	}
	return nil
}

// InputFile builds a tg.InputFileLocationClass suitable for downloading this file.
// Returns nil when the file has no associated location info.
func (f *MCUBFile) InputFile() tg.InputFileLocationClass {
	if doc := f.document(); doc != nil {
		return &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
		}
	}
	if photo := f.photo(); photo != nil {
		// Use the largest available PhotoSize.
		var largest tg.PhotoSizeClass
		for _, sz := range photo.Sizes {
			largest = sz
		}
		loc := &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
		}
		if largest != nil {
			if ps, ok := largest.(*tg.PhotoSize); ok {
				loc.ThumbSize = ps.Type
			}
		}
		return loc
	}
	return nil
}

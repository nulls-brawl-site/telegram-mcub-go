package types

import (
	"time"

	"github.com/gotd/td/tg"
)

// InlineResultType constants mirror Telethon's InlineResult type strings.
const (
	InlineResultArticle  = "article"
	InlineResultPhoto    = "photo"
	InlineResultGIF      = "gif"
	InlineResultVideo    = "video"
	InlineResultVideoGIF = "mpeg4_gif"
	InlineResultAudio    = "audio"
	InlineResultDocument = "document"
	InlineResultLocation = "location"
	InlineResultVenue    = "venue"
	InlineResultContact  = "contact"
	InlineResultGame     = "game"
)

// InlineResult wraps a single bot inline query result.
// It mirrors Telethon's tl/custom/inlineresult.InlineResult class.
type InlineResult struct {
	// Result is the underlying gotd/td BotInlineResult.
	Result tg.BotInlineResultClass

	// QueryID is the inline query ID this result belongs to.
	QueryID int64

	// Entity is the optional pre-resolved target entity.
	Entity interface{}
}

// NewInlineResult creates an InlineResult from a raw BotInlineResultClass.
func NewInlineResult(result tg.BotInlineResultClass, queryID int64) *InlineResult {
	return &InlineResult{Result: result, QueryID: queryID}
}

// Type returns the result type string (e.g. "article", "photo", etc.).
func (r *InlineResult) Type() string {
	switch v := r.Result.(type) {
	case *tg.BotInlineResult:
		return v.Type
	case *tg.BotInlineMediaResult:
		return v.Type
	}
	return ""
}

// Message returns the send_message field — what will be sent when the result is clicked.
func (r *InlineResult) Message() tg.BotInlineMessageClass {
	switch v := r.Result.(type) {
	case *tg.BotInlineResult:
		return v.SendMessage
	case *tg.BotInlineMediaResult:
		return v.SendMessage
	}
	return nil
}

// Title returns the result title, if present.
func (r *InlineResult) Title() string {
	switch v := r.Result.(type) {
	case *tg.BotInlineResult:
		t, _ := v.GetTitle()
		return t
	case *tg.BotInlineMediaResult:
		t, _ := v.GetTitle()
		return t
	}
	return ""
}

// Description returns the result description, if present.
func (r *InlineResult) Description() string {
	switch v := r.Result.(type) {
	case *tg.BotInlineResult:
		d, _ := v.GetDescription()
		return d
	case *tg.BotInlineMediaResult:
		d, _ := v.GetDescription()
		return d
	}
	return ""
}

// URL returns the URL for normal (non-media) results.
func (r *InlineResult) URL() string {
	if v, ok := r.Result.(*tg.BotInlineResult); ok {
		u, _ := v.GetURL()
		return u
	}
	return ""
}

// Photo returns the thumbnail WebDocument for normal results, or the Photo
// for media results.
func (r *InlineResult) Photo() interface{} {
	switch v := r.Result.(type) {
	case *tg.BotInlineResult:
		if thumb, ok := v.GetThumb(); ok {
			return thumb
		}
	case *tg.BotInlineMediaResult:
		if photo, ok := v.GetPhoto(); ok {
			return photo
		}
	}
	return nil
}

// Document returns the content WebDocument for normal results, or the Document
// for media results.
func (r *InlineResult) Document() interface{} {
	switch v := r.Result.(type) {
	case *tg.BotInlineResult:
		if content, ok := v.GetContent(); ok {
			return content
		}
	case *tg.BotInlineMediaResult:
		if doc, ok := v.GetDocument(); ok {
			return doc
		}
	}
	return nil
}

// InlineResults wraps a collection of InlineResult objects along with query
// metadata.  It mirrors Telethon's tl/custom/inlineresults.InlineResults class.
type InlineResults struct {
	// Items is the list of individual results.
	Items []*InlineResult

	// QueryID is the random ID identifying this query.
	QueryID int64

	// CacheTime is the number of seconds the results are considered valid.
	CacheTime int

	// validUntil is the Unix timestamp after which the cache has expired.
	validUntil time.Time

	// Gallery indicates the results should be presented as an image gallery.
	Gallery bool

	// NextOffset is used to fetch the next page of results (may be empty).
	NextOffset string

	// SwitchPM is set when the bot wants to redirect the user to a PM.
	SwitchPM *tg.InlineBotSwitchPM
}

// NewInlineResults wraps a raw tg.MessagesBotResults into an InlineResults.
func NewInlineResults(raw *tg.MessagesBotResults) *InlineResults {
	items := make([]*InlineResult, 0, len(raw.Results))
	for _, res := range raw.Results {
		items = append(items, NewInlineResult(res, raw.QueryID))
	}

	ir := &InlineResults{
		Items:      items,
		QueryID:    raw.QueryID,
		CacheTime:  raw.CacheTime,
		validUntil: time.Now().Add(time.Duration(raw.CacheTime) * time.Second),
		Gallery:    raw.Gallery,
	}

	if no, ok := raw.GetNextOffset(); ok {
		ir.NextOffset = no
	}
	if pm, ok := raw.GetSwitchPm(); ok {
		ir.SwitchPM = &pm
	}

	return ir
}

// ResultsValid reports whether the cached results are still considered valid.
func (ir *InlineResults) ResultsValid() bool {
	return time.Now().Before(ir.validUntil)
}

// Len returns the number of results.
func (ir *InlineResults) Len() int {
	return len(ir.Items)
}

// Get returns the InlineResult at index i, or nil if out of bounds.
func (ir *InlineResults) Get(i int) *InlineResult {
	if i < 0 || i >= len(ir.Items) {
		return nil
	}
	return ir.Items[i]
}

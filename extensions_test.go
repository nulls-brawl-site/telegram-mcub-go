package main

import (
	"fmt"
	"github.com/nulls-brawl-site/telegram-mcub-go/client/extensions"
)

func main() {
	// === HTML Parser Tests ===

	// Bold + italic
	text, ents, _ := extensions.ParseHTML("<b>hello</b> <i>world</i>")
	fmt.Printf("HTML bold+italic: text=%q entities=%d\n", text, len(ents))

	// Pre with language
	text, ents, _ = extensions.ParseHTML(`<pre><code class="language-python">print(1)</code></pre>`)
	fmt.Printf("HTML pre+lang: text=%q entities=%d\n", text, len(ents))

	// tg-emoji
	text, ents, _ = extensions.ParseHTML(`<tg-emoji emoji-id="12345">👋</tg-emoji>`)
	fmt.Printf("HTML tg-emoji: text=%q entities=%d\n", text, len(ents))

	// mailto link
	text, ents, _ = extensions.ParseHTML(`<a href="mailto:test@example.com">test@example.com</a>`)
	fmt.Printf("HTML email: text=%q entities=%d\n", text, len(ents))

	// tg:// mention
	text, ents, _ = extensions.ParseHTML(`<a href="tg://user?id=123456">John</a>`)
	fmt.Printf("HTML mention: text=%q entities=%d\n", text, len(ents))

	// TextUrl auto-convert to URL
	text, ents, _ = extensions.ParseHTML(`<a href="https://example.com">https://example.com</a>`)
	fmt.Printf("HTML url auto-convert: text=%q entities=%d type=%T\n", text, len(ents), ents[0])

	// TextUrl stays as TextUrl when text != href
	text, ents, _ = extensions.ParseHTML(`<a href="https://example.com">click here</a>`)
	fmt.Printf("HTML texturl: text=%q entities=%d type=%T\n", text, len(ents), ents[0])

	// blockquote expandable
	text, ents, _ = extensions.ParseHTML(`<blockquote expandable>hello</blockquote>`)
	fmt.Printf("HTML blockquote expandable: text=%q entities=%d\n", text, len(ents))

	// Invalid tag syntax (like <3)
	text, ents, _ = extensions.ParseHTML("Hello <3 world")
	fmt.Printf("HTML invalid tag: text=%q entities=%d\n", text, len(ents))

	// Unsupported tag preserved literally
	text, ents, _ = extensions.ParseHTML("<div>hello</div>")
	fmt.Printf("HTML unsupported tag: text=%q entities=%d\n", text, len(ents))

	// === Markdown Parser Tests ===
	text, ents, _ = extensions.ParseMarkdown("**bold** __italic__ ~~strike~~")
	fmt.Printf("MD basic: text=%q entities=%d\n", text, len(ents))

	text, ents, _ = extensions.ParseMarkdown("`code` ```pre```")
	fmt.Printf("MD code+pre: text=%q entities=%d\n", text, len(ents))

	text, ents, _ = extensions.ParseMarkdown("[click](https://example.com)")
	fmt.Printf("MD texturl: text=%q entities=%d\n", text, len(ents))

	text, ents, _ = extensions.ParseMarkdown("[John](tg://user?id=123)")
	fmt.Printf("MD mention: text=%q entities=%d\n", text, len(ents))

	// === Unparse HTML ===
	import_ents := ents // just reuse something
	_ = import_ents
	html := extensions.UnparseHTML("hello world", nil)
	fmt.Printf("Unparse HTML (no entities): %q\n", html)

	// === Unparse Markdown ===
	md := extensions.UnparseMarkdown("bold text", nil)
	fmt.Printf("Unparse MD (no entities): %q\n", md)
}

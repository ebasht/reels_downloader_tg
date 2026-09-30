package telegram

import (
	"html"
	"regexp"
	"strings"
	"testing"
)

var tagRe = regexp.MustCompile(`<[^>]+>`)

// visibleLen is the caption length as Telegram counts it after parsing HTML.
func visibleLen(s string) int {
	return utf16Len(html.UnescapeString(tagRe.ReplaceAllString(s, "")))
}

func TestCaptionHTMLShortIsEscaped(t *testing.T) {
	if got := captionHTML("a < b & c"); got != "a &lt; b &amp; c" {
		t.Fatalf("got %q", got)
	}
}

func TestCaptionHTMLLongKeepsHeadAndCollapsesBody(t *testing.T) {
	head := "Audi S4, 1999\n1 050 000 ₽"
	caption := head + "\n\n" + strings.Repeat("текст & ", 300)
	got := captionHTML(caption)

	if !strings.HasPrefix(got, html.EscapeString(head)+"\n<blockquote expandable>") ||
		!strings.HasSuffix(got, "…</blockquote>") {
		t.Fatalf("unexpected layout: %.120q…%q", got, got[len(got)-40:])
	}
	if n := visibleLen(got); n > maxCaptionLen {
		t.Fatalf("visible length %d > %d", n, maxCaptionLen)
	}
}

func TestCaptionHTMLLongWithoutParagraphs(t *testing.T) {
	got := captionHTML(strings.Repeat("😀", 600)) // 2 UTF-16 units each
	if !strings.HasPrefix(got, "<blockquote expandable>😀") {
		t.Fatalf("got %.60q", got)
	}
	if n := visibleLen(got); n > maxCaptionLen {
		t.Fatalf("visible length %d > %d", n, maxCaptionLen)
	}
}

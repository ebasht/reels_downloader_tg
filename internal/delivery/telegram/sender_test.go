package telegram

import (
	"html"
	"regexp"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"video_download_bot/internal/domain"
)

var tagRe = regexp.MustCompile(`<[^>]+>`)

// visibleLen is the caption length as Telegram counts it after parsing HTML.
func visibleLen(s string) int {
	return utf16Len(html.UnescapeString(tagRe.ReplaceAllString(s, "")))
}

var testLink = domain.Link{Kind: domain.LinkListing, Source: domain.SourceAutoRu, URL: "https://auto.ru/cars/used/sale/a/b/1-a/?x=1&y=2"}

func TestCaptionHTMLShortIsEscaped(t *testing.T) {
	if got := captionHTML(header{}, "a < b & c"); got != "a &lt; b &amp; c" {
		t.Fatalf("got %q", got)
	}
}

func TestCaptionHTMLLongKeepsHeadAndCollapsesBody(t *testing.T) {
	head := "Audi S4, 1999\n1 050 000 ₽"
	caption := head + "\n\n" + strings.Repeat("текст & ", 300)
	got := captionHTML(header{}, caption)

	if !strings.HasPrefix(got, html.EscapeString(head)+"\n<blockquote expandable>") ||
		!strings.HasSuffix(got, "…</blockquote>") {
		t.Fatalf("unexpected layout: %.120q…%q", got, got[len(got)-40:])
	}
	if n := visibleLen(got); n > maxCaptionLen {
		t.Fatalf("visible length %d > %d", n, maxCaptionLen)
	}
}

func TestCaptionHTMLLongWithoutParagraphs(t *testing.T) {
	got := captionHTML(header{}, strings.Repeat("😀", 600)) // 2 UTF-16 units each
	if !strings.HasPrefix(got, "<blockquote expandable>😀") {
		t.Fatalf("got %.60q", got)
	}
	if n := visibleLen(got); n > maxCaptionLen {
		t.Fatalf("visible length %d > %d", n, maxCaptionLen)
	}
}

func TestHeader(t *testing.T) {
	from := &tgbotapi.User{ID: 42, FirstName: "Ева", LastName: "<Б>"}
	h := newHeader(testLink, from, "смотри & оцени")

	want := `<a href="https://auto.ru/cars/used/sale/a/b/1-a/?x=1&amp;y=2">auto.ru</a>` +
		` · от <a href="tg://user?id=42">Ева &lt;Б&gt;</a>: смотри &amp; оцени`
	if h.html != want {
		t.Fatalf("got  %q\nwant %q", h.html, want)
	}
	if h.visible != visibleLen(h.html) {
		t.Fatalf("visible = %d, want %d", h.visible, visibleLen(h.html))
	}

	if got := newHeader(testLink, &tgbotapi.User{ID: 1, UserName: "nick"}, "").html; !strings.HasSuffix(got, `>@nick</a>`) {
		t.Fatalf("no comment: %q", got)
	}
}

func TestCaptionHTMLWithHeader(t *testing.T) {
	h := newHeader(testLink, &tgbotapi.User{ID: 1, FirstName: "A"}, strings.Repeat("к", 500))

	if got := captionHTML(h, ""); got != h.html {
		t.Fatalf("header only: %q", got)
	}

	got := captionHTML(h, "Заголовок\n\n"+strings.Repeat("слово ", 400))
	if !strings.HasPrefix(got, h.html+"\n\nЗаголовок\n<blockquote expandable>") {
		t.Fatalf("unexpected layout: %.200q", got)
	}
	if n := visibleLen(got); n > maxCaptionLen {
		t.Fatalf("visible length %d > %d", n, maxCaptionLen)
	}
	if h.visible > maxCommentLen+40 {
		t.Fatalf("comment not truncated: header %d", h.visible)
	}
}

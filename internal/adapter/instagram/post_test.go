package instagram

import (
	"net/url"
	"testing"
)

const samplePage = `<html><head>
<meta property="og:title" content="&#x410;&#x432;&#x442;&#x43e; on Instagram: &quot;&#x41f;&#x440;&#x43e;&#x434;&#x430;&#x435;&#x442;&#x441;&#x44f;
BMW E39&quot;" />
<meta property="og:image" content="https://scontent.cdninstagram.com/v/a.jpg?stp=c256&amp;_nc_cat=101" />
<meta property="og:url" content="https://www.instagram.com/avtorynok_rostov_/p/Ddl-FKUDYTH/" />
<meta property="og:description" content="1,379 likes, 74 comments - avtorynok_rostov_ on September 22, 2026: &quot;&#x41f;&#x440;&#x43e;&#x434;&#x430;&#x435;&#x442;&#x441;&#x44f;
BMW E39
&#x426;&#x435;&#x43d;&#x430;: 2.9&quot;. " />
</head></html>`

func TestParsePostPage(t *testing.T) {
	tags := parseMetaTags(samplePage)

	if got, want := tags["og:image"], "https://scontent.cdninstagram.com/v/a.jpg?stp=c256&_nc_cat=101"; got != want {
		t.Errorf("og:image = %q, want %q", got, want)
	}
	if isVideoPage(tags) {
		t.Error("photo post detected as video")
	}
	if got, want := extractCaption(tags), "Продается\nBMW E39\nЦена: 2.9"; got != want {
		t.Errorf("caption = %q, want %q", got, want)
	}
}

func TestExtractCaptionFallsBackToTitle(t *testing.T) {
	tags := map[string]string{
		"og:description": "10 likes, 0 comments - user on May 1, 2026",
		"og:title":       `User on Instagram: "hello"`,
	}
	if got := extractCaption(tags); got != "hello" {
		t.Errorf("caption = %q, want %q", got, "hello")
	}
}

func TestParseEmbedImageURL(t *testing.T) {
	page := `<div><img class="EmbeddedMediaImage" alt="Instagram post shared by &#064;user" src="https://scontent.cdninstagram.com/v/a.jpg?stp=dst-jpg_e35_tt6&amp;_nc_cat=105" srcset="x" /></div>`
	if got, want := parseEmbedImageURL(page), "https://scontent.cdninstagram.com/v/a.jpg?stp=dst-jpg_e35_tt6&_nc_cat=105"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := parseEmbedImageURL("<html></html>"); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestIsAllowedURL(t *testing.T) {
	tests := map[string]bool{
		"https://www.instagram.com/p/x/":                   true,
		"https://scontent-ams2-1.cdninstagram.com/v/a.jpg": true,
		"https://scontent.xx.fbcdn.net/a.jpg":              true,
		"http://www.instagram.com/p/x/":                    false,
		"https://evil.com/a.jpg":                           false,
		"https://instagram.com.evil.com/a.jpg":             false,
		"https://evilcdninstagram.com/a.jpg":               false,
		"https://169.254.169.254/latest/meta-data":         false,
	}
	for raw, want := range tests {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if got := isAllowedURL(u); got != want {
			t.Errorf("isAllowedURL(%q) = %v, want %v", raw, got, want)
		}
	}
}

func TestIsVideoPage(t *testing.T) {
	tags := map[string]string{"og:url": "https://www.instagram.com/unit_auto/reel/Dd4BAPNsnje/"}
	if !isVideoPage(tags) {
		t.Error("reel page not detected as video")
	}
}

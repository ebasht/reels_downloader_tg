package instagram

import (
	"encoding/json"
	"slices"
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

func TestParseCarouselItems(t *testing.T) {
	doc := `{"context":{"type":"GraphSidecar"},"gql_data":{"shortcode_media":{"__typename":"GraphSidecar",` +
		`"edge_sidecar_to_children":{"edges":[` +
		`{"node":{"is_video":false,"display_url":"https://scontent.cdninstagram.com/1.jpg?a=1\u0026b=2"}},` +
		`{"node":{"is_video":true,"display_url":"https://scontent.cdninstagram.com/cover.jpg",` +
		`"video_url":"https://scontent.cdninstagram.com/v.mp4","dimensions":{"width":720,"height":960}}},` +
		`{"node":{"is_video":true,"display_url":"https://scontent.cdninstagram.com/no-video.jpg"}},` +
		`{"node":{"is_video":false,"display_url":"https://scontent.cdninstagram.com/2.jpg"}}]}}}}`
	encoded, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	page := `<script>{"isSidecar":true,"contextJSON":` + string(encoded) + `,"other":1}</script>`

	got := parseCarouselItems(page)
	want := []embedItem{
		{url: "https://scontent.cdninstagram.com/1.jpg?a=1&b=2"},
		{url: "https://scontent.cdninstagram.com/v.mp4", isVideo: true, width: 720, height: 960},
		{url: "https://scontent.cdninstagram.com/2.jpg"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}

	single := `{"contextJSON":"{\"context\":{\"type\":\"GraphImage\"},\"gql_data\":null}"}`
	if got := parseCarouselItems(single); got != nil {
		t.Fatalf("single post: got %+v, want nil", got)
	}
}

func TestIsVideoPage(t *testing.T) {
	tags := map[string]string{"og:url": "https://www.instagram.com/unit_auto/reel/Dd4BAPNsnje/"}
	if !isVideoPage(tags) {
		t.Error("reel page not detected as video")
	}
}

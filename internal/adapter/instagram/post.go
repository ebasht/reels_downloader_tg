package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"time"

	"video_download_bot/internal/adapter/web"
	"video_download_bot/internal/domain"
)

const (
	maxPageBytes  = 5 * 1024 * 1024
	maxImageBytes = 10 * 1024 * 1024
	maxPostImages = 20 // Instagram's carousel limit
)

var (
	metaTagRe = regexp.MustCompile(`(?s)<meta\s+(?:property|name)="((?:og|twitter):[a-z_:]+)"\s+content="([^"]*)"`)
	// og:description: `1,379 likes, 74 comments - user on September 22, 2026: "caption". `
	descCaptionRe = regexp.MustCompile(`(?s)^[^"]*?:\s*"(.*)"\.?\s*$`)
	// og:title: `Name on Instagram: "caption"`
	titleCaptionRe  = regexp.MustCompile(`(?s)on Instagram:\s*"(.*)"\s*$`)
	embedImageTagRe = regexp.MustCompile(`<img\b[^>]*\bclass="EmbeddedMediaImage"[^>]*>`)
	srcAttrRe       = regexp.MustCompile(`\bsrc="([^"]+)"`)
	contextJSONRe   = regexp.MustCompile(`"contextJSON":("(?:[^"\\]|\\.)*")`)
)

type PostFetcher struct {
	web *web.Client
}

func NewPostFetcher(timeout time.Duration) *PostFetcher {
	return &PostFetcher{web: web.NewClient(timeout, web.UserAgentFacebook, "en-US,en;q=0.9",
		"instagram.com", "cdninstagram.com", "fbcdn.net")}
}

func (f *PostFetcher) FetchPost(ctx context.Context, postURL string) (*domain.Post, error) {
	page, err := f.get(ctx, postURL, maxPageBytes)
	if err != nil {
		return nil, fmt.Errorf("load page: %w", err)
	}

	tags := parseMetaTags(string(page))
	if isVideoPage(tags) {
		return nil, domain.ErrPostIsVideo
	}

	imageURL := tags["og:image"]
	if imageURL == "" {
		return nil, fmt.Errorf("%w: no og:image (private or deleted post?)", domain.ErrMediaNotFound)
	}

	// og:image is a 640x640 square crop of the first photo; the embed page has
	// all carousel photos uncropped.
	var images [][]byte
	for _, u := range f.embedImageURLs(ctx, postURL) {
		img, err := f.get(ctx, u, maxImageBytes)
		if err != nil {
			log.Printf("instagram full-size image failed: %v", err)
			continue
		}
		images = append(images, img)
	}
	if len(images) == 0 {
		img, err := f.get(ctx, imageURL, maxImageBytes)
		if err != nil {
			return nil, fmt.Errorf("load image: %w", err)
		}
		images = [][]byte{img}
	}

	return &domain.Post{
		Images:  images,
		Caption: extractCaption(tags),
	}, nil
}

func (f *PostFetcher) get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	resp, err := f.web.Get(ctx, rawURL, limit)
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

func (f *PostFetcher) embedImageURLs(ctx context.Context, postURL string) []string {
	page, err := f.get(ctx, strings.TrimSuffix(postURL, "/")+"/embed/captioned/", maxPageBytes)
	if err != nil {
		log.Printf("instagram embed page failed: %v", err)
		return nil
	}
	if urls := parseCarouselImageURLs(string(page)); len(urls) > 0 {
		return urls
	}
	if u := parseEmbedImageURL(string(page)); u != "" {
		return []string{u}
	}
	return nil
}

// parseCarouselImageURLs extracts carousel photos from the embed page's
// contextJSON, a JSON document stored as a JSON string. Video items are
// skipped. Returns nil for single-photo posts, which have no carousel data.
func parseCarouselImageURLs(page string) []string {
	m := contextJSONRe.FindStringSubmatch(page)
	if m == nil {
		return nil
	}
	var raw string
	if err := json.Unmarshal([]byte(m[1]), &raw); err != nil {
		return nil
	}
	var doc struct {
		GQLData *struct {
			ShortcodeMedia *struct {
				Sidecar *struct {
					Edges []struct {
						Node struct {
							IsVideo    bool   `json:"is_video"`
							DisplayURL string `json:"display_url"`
						} `json:"node"`
					} `json:"edges"`
				} `json:"edge_sidecar_to_children"`
			} `json:"shortcode_media"`
		} `json:"gql_data"`
	}
	if err := json.Unmarshal([]byte(raw), &doc); err != nil {
		return nil
	}
	if doc.GQLData == nil || doc.GQLData.ShortcodeMedia == nil || doc.GQLData.ShortcodeMedia.Sidecar == nil {
		return nil
	}

	var urls []string
	for _, e := range doc.GQLData.ShortcodeMedia.Sidecar.Edges {
		if e.Node.IsVideo || e.Node.DisplayURL == "" {
			continue
		}
		urls = append(urls, e.Node.DisplayURL)
		if len(urls) == maxPostImages {
			break
		}
	}
	return urls
}

func parseEmbedImageURL(page string) string {
	tag := embedImageTagRe.FindString(page)
	if tag == "" {
		return ""
	}
	m := srcAttrRe.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	return html.UnescapeString(m[1])
}

func parseMetaTags(page string) map[string]string {
	tags := make(map[string]string)
	for _, m := range metaTagRe.FindAllStringSubmatch(page, -1) {
		if _, exists := tags[m[1]]; !exists {
			tags[m[1]] = html.UnescapeString(m[2])
		}
	}
	return tags
}

func isVideoPage(tags map[string]string) bool {
	if tags["og:video"] != "" {
		return true
	}
	u := tags["og:url"]
	return strings.Contains(u, "/reel/") || strings.Contains(u, "/tv/")
}

func extractCaption(tags map[string]string) string {
	if m := descCaptionRe.FindStringSubmatch(tags["og:description"]); m != nil {
		return strings.TrimSpace(m[1])
	}
	if m := titleCaptionRe.FindStringSubmatch(tags["og:title"]); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

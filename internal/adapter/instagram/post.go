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
	// maxPostBytes bounds memory held by one carousel.
	maxPostBytes = 150 * 1024 * 1024
	maxPostItems = 20 // Instagram's carousel limit
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
	// all carousel items uncropped.
	var items []domain.PostItem
	var total int
	for _, it := range f.embedItems(ctx, postURL) {
		limit := int64(maxImageBytes)
		if it.isVideo {
			limit = maxVideoBytes
		}
		data, err := f.get(ctx, it.url, limit)
		if err != nil {
			log.Printf("instagram carousel item failed: %v", err)
			continue
		}
		if total += len(data); total > maxPostBytes {
			log.Printf("instagram post exceeds %d bytes, dropping the rest", maxPostBytes)
			break
		}
		items = append(items, domain.PostItem{Data: data, IsVideo: it.isVideo, Width: it.width, Height: it.height})
	}
	if len(items) == 0 {
		img, err := f.get(ctx, imageURL, maxImageBytes)
		if err != nil {
			return nil, fmt.Errorf("load image: %w", err)
		}
		items = []domain.PostItem{{Data: img}}
	}

	return &domain.Post{
		Items:   items,
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

// embedItem is a carousel photo or video as listed on the embed page.
type embedItem struct {
	url           string
	isVideo       bool
	width, height int
}

func (f *PostFetcher) embedItems(ctx context.Context, postURL string) []embedItem {
	page, err := f.get(ctx, strings.TrimSuffix(postURL, "/")+"/embed/captioned/", maxPageBytes)
	if err != nil {
		log.Printf("instagram embed page failed: %v", err)
		return nil
	}
	if items := parseCarouselItems(string(page)); len(items) > 0 {
		return items
	}
	if u := parseEmbedImageURL(string(page)); u != "" {
		return []embedItem{{url: u}}
	}
	return nil
}

// parseCarouselItems extracts carousel photos and videos, in order, from the
// embed page's contextJSON, a JSON document stored as a JSON string. Returns
// nil for single-photo posts, which have no carousel data.
func parseCarouselItems(page string) []embedItem {
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
							VideoURL   string `json:"video_url"`
							Dimensions struct {
								Width  int `json:"width"`
								Height int `json:"height"`
							} `json:"dimensions"`
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

	var items []embedItem
	for _, e := range doc.GQLData.ShortcodeMedia.Sidecar.Edges {
		n := e.Node
		item := embedItem{url: n.DisplayURL}
		if n.IsVideo {
			item = embedItem{url: n.VideoURL, isVideo: true, width: n.Dimensions.Width, height: n.Dimensions.Height}
		}
		if item.url == "" {
			continue
		}
		items = append(items, item)
		if len(items) == maxPostItems {
			break
		}
	}
	return items
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

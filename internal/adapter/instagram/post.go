package instagram

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"video_download_bot/internal/domain"
)

const (
	// Instagram serves full OpenGraph tags (first image + caption) to link
	// preview crawlers without requiring a login.
	crawlerUserAgent = "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)"
	maxPageBytes     = 5 * 1024 * 1024
	maxImageBytes    = 10 * 1024 * 1024
)

var (
	metaTagRe = regexp.MustCompile(`(?s)<meta\s+(?:property|name)="((?:og|twitter):[a-z_:]+)"\s+content="([^"]*)"`)
	// og:description: `1,379 likes, 74 comments - user on September 22, 2026: "caption". `
	descCaptionRe = regexp.MustCompile(`(?s)^[^"]*?:\s*"(.*)"\.?\s*$`)
	// og:title: `Name on Instagram: "caption"`
	titleCaptionRe  = regexp.MustCompile(`(?s)on Instagram:\s*"(.*)"\s*$`)
	embedImageTagRe = regexp.MustCompile(`<img\b[^>]*\bclass="EmbeddedMediaImage"[^>]*>`)
	srcAttrRe       = regexp.MustCompile(`\bsrc="([^"]+)"`)
)

type PostFetcher struct {
	client *http.Client
}

func NewPostFetcher(timeout time.Duration) *PostFetcher {
	return &PostFetcher{client: &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !isAllowedURL(req.URL) {
				return fmt.Errorf("redirect to disallowed host %q", req.URL.Host)
			}
			return nil
		},
	}}
}

// isAllowedURL restricts fetches to Instagram and its CDNs: image URLs come
// from page HTML and must not point the bot at arbitrary hosts.
func isAllowedURL(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, domain := range []string{"instagram.com", "cdninstagram.com", "fbcdn.net"} {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
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

	// og:image is a 640x640 square crop; the embed page has the uncropped first photo.
	var image []byte
	if fullURL := f.embedImageURL(ctx, postURL); fullURL != "" {
		if image, err = f.get(ctx, fullURL, maxImageBytes); err != nil {
			log.Printf("instagram full-size image failed, using og:image: %v", err)
			image = nil
		}
	}
	if image == nil {
		if image, err = f.get(ctx, imageURL, maxImageBytes); err != nil {
			return nil, fmt.Errorf("load image: %w", err)
		}
	}

	return &domain.Post{
		Image:   image,
		Caption: extractCaption(tags),
	}, nil
}

func (f *PostFetcher) get(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if !isAllowedURL(req.URL) {
		return nil, fmt.Errorf("disallowed host %q", req.URL.Host)
	}
	req.Header.Set("User-Agent", crawlerUserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return body, nil
}

func (f *PostFetcher) embedImageURL(ctx context.Context, postURL string) string {
	page, err := f.get(ctx, strings.TrimSuffix(postURL, "/")+"/embed/captioned/", maxPageBytes)
	if err != nil {
		log.Printf("instagram embed page failed: %v", err)
		return ""
	}
	return parseEmbedImageURL(string(page))
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

// Package web fetches pages and images from a fixed set of sites.
package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/text/encoding/charmap"
)

// Link-preview crawlers get full server-rendered pages from the supported
// sites, while regular browser user agents get a JS challenge or captcha.
const (
	UserAgentFacebook = "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)"
	UserAgentTelegram = "TelegramBot (like TwitterBot)"
)

// Client only talks HTTPS to the allowed domains and their subdomains: image
// URLs come from page HTML and must not point the bot at arbitrary hosts.
type Client struct {
	http      *http.Client
	userAgent string
	language  string
	domains   []string
}

// NewClient creates a client sending userAgent and the Accept-Language language.
func NewClient(timeout time.Duration, userAgent, language string, domains ...string) *Client {
	c := &Client{userAgent: userAgent, language: language, domains: domains}
	c.http = &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("too many redirects")
			}
			if !c.Allowed(req.URL) {
				return fmt.Errorf("redirect to disallowed host %q", req.URL.Host)
			}
			return nil
		},
	}
	return c
}

func (c *Client) Allowed(u *url.URL) bool {
	if u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, d := range c.domains {
		if host == d || strings.HasSuffix(host, "."+d) {
			return true
		}
	}
	return false
}

type Response struct {
	Body        []byte
	ContentType string
}

// Get fetches rawURL, failing on non-200 responses and bodies over limit bytes.
func (c *Client) Get(ctx context.Context, rawURL string, limit int64) (Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Response{}, err
	}
	if !c.Allowed(req.URL) {
		return Response{}, fmt.Errorf("disallowed host %q", req.URL.Host)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept-Language", c.language)

	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Response{}, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return Response{}, err
	}
	if int64(len(body)) > limit {
		return Response{}, fmt.Errorf("response exceeds %d bytes", limit)
	}
	return Response{Body: body, ContentType: resp.Header.Get("Content-Type")}, nil
}

// Text returns the body as UTF-8, decoding windows-1251 pages.
func (r Response) Text() (string, error) {
	ct := strings.ToLower(r.ContentType)
	if strings.Contains(ct, "windows-1251") || strings.Contains(ct, "cp1251") {
		b, err := charmap.Windows1251.NewDecoder().Bytes(r.Body)
		if err != nil {
			return "", fmt.Errorf("decode windows-1251: %w", err)
		}
		return string(b), nil
	}
	return string(r.Body), nil
}

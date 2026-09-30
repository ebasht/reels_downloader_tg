package domain

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var (
	ErrMediaNotFound = errors.New("media not found")
	ErrPostIsVideo   = errors.New("post contains video")
)

type LinkKind int

const (
	LinkReel LinkKind = iota + 1
	LinkPost
)

type InstagramLink struct {
	Kind      LinkKind
	Shortcode string
	URL       string
}

var instagramLinkRe = regexp.MustCompile(
	`(?i)https?://(?:www\.|m\.)?instagram\.com/(?:[A-Za-z0-9_.]+/)?(p|reels?|tv)/([A-Za-z0-9_-]+)`,
)

// FindInstagramLink returns the first reel or post link found in text.
func FindInstagramLink(text string) (InstagramLink, bool) {
	m := instagramLinkRe.FindStringSubmatch(text)
	if m == nil {
		return InstagramLink{}, false
	}

	kind, path := LinkReel, "reel"
	if strings.EqualFold(m[1], "p") {
		kind, path = LinkPost, "p"
	}
	return InstagramLink{
		Kind:      kind,
		Shortcode: m[2],
		URL:       fmt.Sprintf("https://www.instagram.com/%s/%s/", path, m[2]),
	}, true
}

// Video is a downloaded video file ready to be uploaded to Telegram.
type Video struct {
	Path          string
	ThumbnailPath string
	Width         int
	Height        int
	Duration      int
	// Release frees the underlying files. Safe to call multiple times.
	Release func()
}

// Post is the first photo of an Instagram post with its caption.
type Post struct {
	Image   []byte
	Caption string
}

// Media holds exactly one of Video or Post.
type Media struct {
	Video *Video
	Post  *Post
}

func (m Media) Release() {
	if m.Video != nil && m.Video.Release != nil {
		m.Video.Release()
	}
}

package domain

import "time"

type MediaType string

const (
	MediaReel MediaType = "reel"
	MediaPost MediaType = "post"
)

// Type reports what was actually delivered: a video counts as a reel even if
// it came from a /p/ link.
func (m Media) Type() MediaType {
	if m.Video != nil {
		return MediaReel
	}
	return MediaPost
}

// Download is a link successfully delivered to a chat.
type Download struct {
	ChatID    int64
	User      User
	URL       string
	Shortcode string
	Type      MediaType
	At        time.Time
}

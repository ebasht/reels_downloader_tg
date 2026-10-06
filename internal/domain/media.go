package domain

import "errors"

var (
	ErrMediaNotFound = errors.New("media not found")
	ErrPostIsVideo   = errors.New("post contains video")
	ErrVideoTooLong  = errors.New("video is too long")
)

// Video is a downloaded file ready to send.
type Video struct {
	Path          string
	ThumbnailPath string
	Width         int
	Height        int
	Duration      int
	// Release frees the underlying files. Safe to call multiple times.
	Release func()
}

// Post holds photos and videos with a caption: an Instagram post (all items
// of a carousel) or a car listing with its ad text.
type Post struct {
	Items   []PostItem
	Caption string
}

// PostItem is a downloaded photo or video.
type PostItem struct {
	Data    []byte
	IsVideo bool
	// Width and Height are set for videos when known.
	Width  int
	Height int
}

// Media holds exactly one of Video or Post.
type Media struct {
	Video *Video
	Post  *Post
}

// Release frees the downloaded files and data. Safe to call multiple times.
func (m Media) Release() {
	if m.Video != nil && m.Video.Release != nil {
		m.Video.Release()
	}
	if m.Post != nil {
		m.Post.Items = nil
	}
}

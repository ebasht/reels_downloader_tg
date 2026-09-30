package domain

import "errors"

var (
	ErrMediaNotFound = errors.New("media not found")
	ErrPostIsVideo   = errors.New("post contains video")
	ErrVideoTooLong  = errors.New("video is too long")
)

// Video is either a downloaded file (Path) or a file already stored on
// Telegram servers (FileID).
type Video struct {
	Path          string
	ThumbnailPath string
	FileID        string
	Width         int
	Height        int
	Duration      int
	// Release frees the underlying files. Safe to call multiple times.
	Release func()
}

// Post holds photos with a caption: an Instagram post (all photos of a
// carousel) or a car listing with its ad text. Photos are either downloaded
// bytes (Images) or files on Telegram servers (ImageFileIDs).
type Post struct {
	Images       [][]byte
	ImageFileIDs []string
	Caption      string
}

// Media holds exactly one of Video or Post.
type Media struct {
	Video *Video
	Post  *Post
	// Cached is true when the media was taken from the cache of files already
	// sent to Telegram rather than downloaded from Instagram.
	Cached bool
}

// FileIDs returns the Telegram file IDs of the media, if it has them.
func (m Media) FileIDs() []string {
	switch {
	case m.Video != nil && m.Video.FileID != "":
		return []string{m.Video.FileID}
	case m.Post != nil:
		return m.Post.ImageFileIDs
	default:
		return nil
	}
}

func (m Media) Release() {
	if m.Video != nil && m.Video.Release != nil {
		m.Video.Release()
	}
}

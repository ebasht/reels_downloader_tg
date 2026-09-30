package telegram

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"unicode/utf16"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"video_download_bot/internal/domain"
)

// Telegram measures text limits in UTF-16 code units.
const (
	maxCaptionLen = 1024
	maxAlbumSize  = 10
	// maxHeadLen caps the paragraph kept outside the collapsed quote.
	maxHeadLen = 300
)

// sendVideo sends the video and returns its Telegram file ID.
func (h *Handler) sendVideo(msg *tgbotapi.Message, v *domain.Video) (string, error) {
	// VideoConfig in tgbotapi v5 has no Width/Height fields, but Telegram needs them
	// for correct aspect in the in-app player — send via UploadFiles.
	params := tgbotapi.Params{}
	params.AddNonZero64("chat_id", msg.Chat.ID)
	params.AddNonZero("reply_to_message_id", msg.MessageID)
	params.AddBool("allow_sending_without_reply", true)
	params.AddNonZero("width", v.Width)
	params.AddNonZero("height", v.Height)
	params.AddNonZero("duration", v.Duration)
	params.AddBool("supports_streaming", true)

	var files []tgbotapi.RequestFile
	if v.FileID != "" {
		files = append(files, tgbotapi.RequestFile{Name: "video", Data: tgbotapi.FileID(v.FileID)})
	} else {
		files = append(files, tgbotapi.RequestFile{Name: "video", Data: tgbotapi.FilePath(v.Path)})
		if v.ThumbnailPath != "" {
			files = append(files, tgbotapi.RequestFile{Name: "thumb", Data: tgbotapi.FilePath(v.ThumbnailPath)})
		}
	}

	resp, err := h.api.UploadFiles("sendVideo", params, files)
	if err != nil {
		return "", err
	}
	var sent tgbotapi.Message
	if err := json.Unmarshal(resp.Result, &sent); err != nil {
		return "", fmt.Errorf("decode sendVideo result: %w", err)
	}
	if sent.Video == nil {
		return "", nil
	}
	return sent.Video.FileID, nil
}

// sendPost sends the post's photos as one message (an album for carousels)
// with the caption and returns the photos' Telegram file IDs in order.
func (h *Handler) sendPost(msg *tgbotapi.Message, p *domain.Post) ([]string, error) {
	files := postFiles(p)
	if len(files) == 0 {
		return nil, fmt.Errorf("post has no photos")
	}

	caption := captionHTML(p.Caption)

	var fileIDs []string
	for start := 0; start < len(files); start += maxAlbumSize {
		chunk := files[start:min(start+maxAlbumSize, len(files))]
		chunkCaption := ""
		if start == 0 {
			chunkCaption = caption
		}

		sent, err := h.sendPhotos(msg, chunk, chunkCaption)
		if err != nil {
			return fileIDs, err
		}
		for _, m := range sent {
			if n := len(m.Photo); n > 0 {
				fileIDs = append(fileIDs, m.Photo[n-1].FileID)
			}
		}
	}
	return fileIDs, nil
}

// captionHTML renders the caption as Telegram HTML. A caption over the limit
// keeps its first paragraph visible and puts the rest, truncated, into an
// expandable blockquote.
func captionHTML(caption string) string {
	if utf16Len(caption) <= maxCaptionLen {
		return html.EscapeString(caption)
	}

	head, body := "", caption
	if h, b, ok := strings.Cut(caption, "\n\n"); ok && utf16Len(h) <= maxHeadLen {
		head, body = h, b
	}
	// The visible text is head + "\n" + body; tags don't count toward the limit.
	budget := maxCaptionLen
	if head != "" {
		budget -= utf16Len(head) + 1
	}
	quote := "<blockquote expandable>" + html.EscapeString(truncateText(body, budget)) + "</blockquote>"
	if head == "" {
		return quote
	}
	return html.EscapeString(head) + "\n" + quote
}

// truncateText cuts s to at most limit UTF-16 units, ending with an ellipsis.
func truncateText(s string, limit int) string {
	if utf16Len(s) <= limit {
		return s
	}
	size := 0
	runes := []rune(s)
	cut := 0
	for cut < len(runes) {
		w := len(utf16.Encode(runes[cut : cut+1]))
		if size+w > limit-1 {
			break
		}
		size += w
		cut++
	}
	return strings.TrimRight(string(runes[:cut]), " \n\t,.;:") + "…"
}

// sendPhotos sends up to maxAlbumSize photos; albums need at least two.
func (h *Handler) sendPhotos(msg *tgbotapi.Message, files []tgbotapi.RequestFileData, caption string) ([]tgbotapi.Message, error) {
	if len(files) == 1 {
		photo := tgbotapi.NewPhoto(msg.Chat.ID, files[0])
		photo.ReplyToMessageID = msg.MessageID
		photo.AllowSendingWithoutReply = true
		photo.Caption = caption
		photo.ParseMode = tgbotapi.ModeHTML
		sent, err := h.api.Send(photo)
		if err != nil {
			return nil, err
		}
		return []tgbotapi.Message{sent}, nil
	}

	items := make([]interface{}, len(files))
	for i, f := range files {
		item := tgbotapi.NewInputMediaPhoto(f)
		if i == 0 {
			item.Caption = caption
			item.ParseMode = tgbotapi.ModeHTML
		}
		items[i] = item
	}
	album := tgbotapi.NewMediaGroup(msg.Chat.ID, items)
	album.ReplyToMessageID = msg.MessageID
	return h.api.SendMediaGroup(album)
}

func postFiles(p *domain.Post) []tgbotapi.RequestFileData {
	var files []tgbotapi.RequestFileData
	if len(p.ImageFileIDs) > 0 {
		for _, id := range p.ImageFileIDs {
			files = append(files, tgbotapi.FileID(id))
		}
		return files
	}
	for i, img := range p.Images {
		files = append(files, tgbotapi.FileBytes{Name: fmt.Sprintf("photo%d.jpg", i+1), Bytes: img})
	}
	return files
}

func (h *Handler) sendMedia(msg *tgbotapi.Message, m domain.Media) ([]string, error) {
	switch {
	case m.Video != nil:
		id, err := h.sendVideo(msg, m.Video)
		if err != nil || id == "" {
			return nil, err
		}
		return []string{id}, nil
	case m.Post != nil:
		return h.sendPost(msg, m.Post)
	default:
		return nil, fmt.Errorf("empty media")
	}
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

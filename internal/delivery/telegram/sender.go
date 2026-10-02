package telegram

import (
	"encoding/json"
	"fmt"
	"html"
	"strconv"
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
	// maxCommentLen caps the sender's own text repeated in the header.
	maxCommentLen = 300
)

// delivery says where the media goes and what header its caption starts with.
type delivery struct {
	chatID int64
	// replyTo is the message to reply to; 0 sends without a reply.
	replyTo int
	header  header
}

// header is the caption's first line: source link, sender and their comment.
type header struct {
	html string
	// visible is the length Telegram counts toward the caption limit.
	visible int
}

func newHeader(link domain.Link, from *tgbotapi.User, comment string) header {
	var b, plain strings.Builder
	site := link.Source.Name()
	fmt.Fprintf(&b, `<a href="%s">%s</a>`, html.EscapeString(link.URL), html.EscapeString(site))
	plain.WriteString(site)

	if from != nil {
		name := displayName(from)
		fmt.Fprintf(&b, ` · от <a href="tg://user?id=%s">%s</a>`, strconv.FormatInt(from.ID, 10), html.EscapeString(name))
		plain.WriteString(" · от " + name)
	}
	if comment != "" {
		comment = truncateText(comment, maxCommentLen)
		b.WriteString(": " + html.EscapeString(comment))
		plain.WriteString(": " + comment)
	}
	return header{html: b.String(), visible: utf16Len(plain.String())}
}

func displayName(u *tgbotapi.User) string {
	if name := strings.TrimSpace(u.FirstName + " " + u.LastName); name != "" {
		return name
	}
	if u.UserName != "" {
		return "@" + u.UserName
	}
	return "пользователь"
}

// sendVideo sends the video and returns its Telegram file ID.
func (h *Handler) sendVideo(d delivery, v *domain.Video) (string, error) {
	// VideoConfig in tgbotapi v5 has no Width/Height fields, but Telegram needs them
	// for correct aspect in the in-app player — send via UploadFiles.
	params := tgbotapi.Params{}
	params.AddNonZero64("chat_id", d.chatID)
	params.AddNonZero("reply_to_message_id", d.replyTo)
	params.AddBool("allow_sending_without_reply", true)
	params.AddNonEmpty("caption", captionHTML(d.header, ""))
	params.AddNonEmpty("parse_mode", tgbotapi.ModeHTML)
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
func (h *Handler) sendPost(d delivery, p *domain.Post) ([]string, error) {
	files := postFiles(p)
	if len(files) == 0 {
		return nil, fmt.Errorf("post has no photos")
	}

	caption := captionHTML(d.header, p.Caption)

	var fileIDs []string
	for start := 0; start < len(files); start += maxAlbumSize {
		chunk := files[start:min(start+maxAlbumSize, len(files))]
		chunkCaption := ""
		if start == 0 {
			chunkCaption = caption
		}

		sent, err := h.sendPhotos(d, chunk, chunkCaption)
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

// captionHTML renders the header followed by the content as Telegram HTML,
// fitting both into the caption limit.
func captionHTML(h header, content string) string {
	limit := maxCaptionLen
	prefix := ""
	if h.html != "" {
		prefix, limit = h.html, limit-h.visible
		if content != "" {
			prefix += "\n\n"
			limit -= 2
		}
	}
	return prefix + contentHTML(content, limit)
}

// contentHTML renders text as Telegram HTML within limit visible units. Text
// over the limit keeps its first paragraph visible and puts the rest,
// truncated, into an expandable blockquote.
func contentHTML(text string, limit int) string {
	if utf16Len(text) <= limit {
		return html.EscapeString(text)
	}

	head, body := "", text
	if h, b, ok := strings.Cut(text, "\n\n"); ok && utf16Len(h) <= maxHeadLen && utf16Len(h) < limit/2 {
		head, body = h, b
	}
	// The visible text is head + "\n" + body; tags don't count toward the limit.
	budget := limit
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
func (h *Handler) sendPhotos(d delivery, files []tgbotapi.RequestFileData, caption string) ([]tgbotapi.Message, error) {
	if len(files) == 1 {
		photo := tgbotapi.NewPhoto(d.chatID, files[0])
		photo.ReplyToMessageID = d.replyTo
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
	album := tgbotapi.NewMediaGroup(d.chatID, items)
	album.ReplyToMessageID = d.replyTo
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

func (h *Handler) sendMedia(d delivery, m domain.Media) ([]string, error) {
	switch {
	case m.Video != nil:
		id, err := h.sendVideo(d, m.Video)
		if err != nil || id == "" {
			return nil, err
		}
		return []string{id}, nil
	case m.Post != nil:
		return h.sendPost(d, m.Post)
	default:
		return nil, fmt.Errorf("empty media")
	}
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

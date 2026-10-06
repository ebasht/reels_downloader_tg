package telegram

import (
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

func (h *Handler) sendVideo(d delivery, v *domain.Video) error {
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

	files := []tgbotapi.RequestFile{{Name: "video", Data: tgbotapi.FilePath(v.Path)}}
	if v.ThumbnailPath != "" {
		files = append(files, tgbotapi.RequestFile{Name: "thumb", Data: tgbotapi.FilePath(v.ThumbnailPath)})
	}
	_, err := h.api.UploadFiles("sendVideo", params, files)
	return err
}

// sendPost sends the post's photos and videos as one message (an album for
// carousels) with the caption.
func (h *Handler) sendPost(d delivery, p *domain.Post) error {
	if len(p.Items) == 0 {
		return fmt.Errorf("post has no photos")
	}

	caption := captionHTML(d.header, p.Caption)
	for start := 0; start < len(p.Items); start += maxAlbumSize {
		chunk := p.Items[start:min(start+maxAlbumSize, len(p.Items))]
		chunkCaption := ""
		if start == 0 {
			chunkCaption = caption
		}
		if err := h.sendItems(d, chunk, start, chunkCaption); err != nil {
			return err
		}
	}
	return nil
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

// sendItems sends up to maxAlbumSize photos and videos; albums need at least
// two items. offset numbers the uploaded files across chunks.
func (h *Handler) sendItems(d delivery, items []domain.PostItem, offset int, caption string) error {
	if len(items) == 1 {
		var cfg tgbotapi.Chattable
		file := itemFile(items[0], offset)
		if items[0].IsVideo {
			video := tgbotapi.NewVideo(d.chatID, file)
			video.ReplyToMessageID = d.replyTo
			video.AllowSendingWithoutReply = true
			video.Caption = caption
			video.ParseMode = tgbotapi.ModeHTML
			video.SupportsStreaming = true
			cfg = video
		} else {
			photo := tgbotapi.NewPhoto(d.chatID, file)
			photo.ReplyToMessageID = d.replyTo
			photo.AllowSendingWithoutReply = true
			photo.Caption = caption
			photo.ParseMode = tgbotapi.ModeHTML
			cfg = photo
		}
		_, err := h.api.Send(cfg)
		return err
	}

	media := make([]interface{}, len(items))
	for i, it := range items {
		file := itemFile(it, offset+i)
		itemCaption := ""
		if i == 0 {
			itemCaption = caption
		}
		if it.IsVideo {
			v := tgbotapi.NewInputMediaVideo(file)
			v.Caption, v.ParseMode = itemCaption, tgbotapi.ModeHTML
			v.Width, v.Height = it.Width, it.Height
			v.SupportsStreaming = true
			media[i] = v
		} else {
			p := tgbotapi.NewInputMediaPhoto(file)
			p.Caption, p.ParseMode = itemCaption, tgbotapi.ModeHTML
			media[i] = p
		}
	}
	album := tgbotapi.NewMediaGroup(d.chatID, media)
	album.ReplyToMessageID = d.replyTo
	_, err := h.api.SendMediaGroup(album)
	return err
}

func itemFile(it domain.PostItem, index int) tgbotapi.RequestFileData {
	ext := "jpg"
	if it.IsVideo {
		ext = "mp4"
	}
	return tgbotapi.FileBytes{Name: fmt.Sprintf("item%d.%s", index+1, ext), Bytes: it.Data}
}

func (h *Handler) sendMedia(d delivery, m domain.Media) error {
	switch {
	case m.Video != nil:
		return h.sendVideo(d, m.Video)
	case m.Post != nil:
		return h.sendPost(d, m.Post)
	default:
		return fmt.Errorf("empty media")
	}
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

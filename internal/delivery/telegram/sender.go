package telegram

import (
	"unicode/utf16"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"video_download_bot/internal/domain"
)

// Telegram measures text limits in UTF-16 code units.
const (
	maxCaptionLen = 1024
	maxMessageLen = 4096
)

func (h *Handler) sendVideo(msg *tgbotapi.Message, v *domain.Video) error {
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

	files := []tgbotapi.RequestFile{{
		Name: "video",
		Data: tgbotapi.FilePath(v.Path),
	}}
	if v.ThumbnailPath != "" {
		files = append(files, tgbotapi.RequestFile{
			Name: "thumb",
			Data: tgbotapi.FilePath(v.ThumbnailPath),
		})
	}

	_, err := h.api.UploadFiles("sendVideo", params, files)
	return err
}

func (h *Handler) sendPost(msg *tgbotapi.Message, p *domain.Post) error {
	photo := tgbotapi.NewPhoto(msg.Chat.ID, tgbotapi.FileBytes{Name: "photo.jpg", Bytes: p.Image})
	photo.ReplyToMessageID = msg.MessageID
	photo.AllowSendingWithoutReply = true

	fitsCaption := utf16Len(p.Caption) <= maxCaptionLen
	if fitsCaption {
		photo.Caption = p.Caption
	}

	sent, err := h.api.Send(photo)
	if err != nil {
		return err
	}
	if fitsCaption {
		return nil
	}

	for _, chunk := range splitText(p.Caption, maxMessageLen) {
		text := tgbotapi.NewMessage(msg.Chat.ID, chunk)
		text.ReplyToMessageID = sent.MessageID
		text.AllowSendingWithoutReply = true
		text.DisableWebPagePreview = true
		if _, err := h.api.Send(text); err != nil {
			return err
		}
	}
	return nil
}

func utf16Len(s string) int {
	return len(utf16.Encode([]rune(s)))
}

// splitText cuts s into chunks of at most limit UTF-16 units, preferring
// to break on newlines.
func splitText(s string, limit int) []string {
	var chunks []string
	runes := []rune(s)
	for len(runes) > 0 {
		size, cut, lastNewline := 0, 0, -1
		for cut < len(runes) {
			w := len(utf16.Encode(runes[cut : cut+1]))
			if size+w > limit {
				break
			}
			size += w
			if runes[cut] == '\n' {
				lastNewline = cut
			}
			cut++
		}
		if cut < len(runes) && lastNewline > 0 {
			cut = lastNewline + 1
		}
		chunks = append(chunks, string(runes[:cut]))
		runes = runes[cut:]
	}
	return chunks
}

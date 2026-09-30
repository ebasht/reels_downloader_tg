package telegram

import (
	"encoding/json"
	"fmt"
	"unicode/utf16"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"video_download_bot/internal/domain"
)

// Telegram measures text limits in UTF-16 code units.
const (
	maxCaptionLen = 1024
	maxMessageLen = 4096
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

// sendPost sends the photo with its caption and returns the photo's Telegram file ID.
func (h *Handler) sendPost(msg *tgbotapi.Message, p *domain.Post) (string, error) {
	var file tgbotapi.RequestFileData = tgbotapi.FileBytes{Name: "photo.jpg", Bytes: p.Image}
	if p.ImageFileID != "" {
		file = tgbotapi.FileID(p.ImageFileID)
	}
	photo := tgbotapi.NewPhoto(msg.Chat.ID, file)
	photo.ReplyToMessageID = msg.MessageID
	photo.AllowSendingWithoutReply = true

	fitsCaption := utf16Len(p.Caption) <= maxCaptionLen
	if fitsCaption {
		photo.Caption = p.Caption
	}

	sent, err := h.api.Send(photo)
	if err != nil {
		return "", err
	}
	var fileID string
	if n := len(sent.Photo); n > 0 {
		fileID = sent.Photo[n-1].FileID
	}
	if fitsCaption {
		return fileID, nil
	}

	for _, chunk := range splitText(p.Caption, maxMessageLen) {
		text := tgbotapi.NewMessage(msg.Chat.ID, chunk)
		text.ReplyToMessageID = sent.MessageID
		text.AllowSendingWithoutReply = true
		text.DisableWebPagePreview = true
		if _, err := h.api.Send(text); err != nil {
			return fileID, err
		}
	}
	return fileID, nil
}

func (h *Handler) sendMedia(msg *tgbotapi.Message, m domain.Media) (string, error) {
	switch {
	case m.Video != nil:
		return h.sendVideo(msg, m.Video)
	case m.Post != nil:
		return h.sendPost(msg, m.Post)
	default:
		return "", fmt.Errorf("empty media")
	}
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

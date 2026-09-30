package telegram

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"video_download_bot/internal/domain"
	"video_download_bot/internal/usecase"
)

const (
	statusText    = "Погоди, скачиваю...."
	rateLimitText = "Слишком много ссылок, попробуй чуть позже."
	busyText      = "Сейчас много загрузок, попробуй чуть позже."
)

type Limits struct {
	MaxConcurrent int
	// MaxQueue caps links waiting for a download slot; extra links are rejected.
	MaxQueue int
	PerUser  *usecase.RateLimiter
	PerChat  *usecase.RateLimiter
}

type Handler struct {
	api     *tgbotapi.BotAPI
	media   *usecase.MediaService
	members *usecase.MembershipService
	limits  Limits
	slots   chan struct{}
	pending atomic.Int64
	wg      sync.WaitGroup
}

func NewHandler(api *tgbotapi.BotAPI, media *usecase.MediaService, members *usecase.MembershipService, limits Limits) *Handler {
	return &Handler{
		api:     api,
		media:   media,
		members: members,
		limits:  limits,
		slots:   make(chan struct{}, limits.MaxConcurrent),
	}
}

// Run polls Telegram for updates until ctx is cancelled, then waits for
// in-flight downloads to finish.
func (h *Handler) Run(ctx context.Context) {
	cfg := tgbotapi.NewUpdate(0)
	cfg.Timeout = 60
	cfg.AllowedUpdates = []string{"message", "my_chat_member"}
	updates := h.api.GetUpdatesChan(cfg)

	defer h.wg.Wait()
	for {
		select {
		case <-ctx.Done():
			h.api.StopReceivingUpdates()
			return
		case upd, ok := <-updates:
			if !ok {
				return
			}
			h.dispatch(ctx, upd)
		}
	}
}

func (h *Handler) dispatch(ctx context.Context, upd tgbotapi.Update) {
	switch {
	case upd.MyChatMember != nil:
		h.handleMyChatMember(ctx, upd.MyChatMember)
	case upd.Message != nil:
		h.handleMessage(ctx, upd.Message)
	}
}

func (h *Handler) handleMyChatMember(ctx context.Context, u *tgbotapi.ChatMemberUpdated) {
	change := domain.MembershipChange{
		Chat:      toDomainChat(&u.Chat),
		Actor:     toDomainUser(u.From),
		OldStatus: domain.MemberStatus(u.OldChatMember.Status),
		NewStatus: domain.MemberStatus(u.NewChatMember.Status),
		At:        time.Unix(int64(u.Date), 0),
	}
	log.Printf("bot membership in chat %d (%s): %s -> %s by user %d",
		change.Chat.ID, change.Chat.Title, change.OldStatus, change.NewStatus, change.Actor.ID)

	if err := h.members.HandleBotMembership(ctx, change); err != nil {
		log.Printf("membership: %v", err)
	}
}

func (h *Handler) handleMessage(ctx context.Context, msg *tgbotapi.Message) {
	if msg.From != nil && msg.From.IsBot {
		return
	}
	if err := h.members.TouchChat(ctx, toDomainChat(msg.Chat)); err != nil {
		log.Printf("touch chat: %v", err)
	}

	text := msg.Text
	if text == "" {
		text = msg.Caption
	}
	link, ok := domain.FindLink(text)
	if !ok {
		return
	}

	if !h.allow(msg) {
		return
	}
	if h.pending.Add(1) > int64(h.limits.MaxQueue) {
		h.pending.Add(-1)
		log.Printf("queue full, dropping link in chat %d", msg.Chat.ID)
		h.reply(msg, busyText)
		return
	}

	h.wg.Add(1)
	go func() {
		defer h.wg.Done()
		defer h.pending.Add(-1)
		select {
		case h.slots <- struct{}{}:
			defer func() { <-h.slots }()
		case <-ctx.Done():
			return
		}
		h.processLink(ctx, msg, link)
	}()
}

func (h *Handler) processLink(ctx context.Context, msg *tgbotapi.Message, link domain.Link) {
	log.Printf("link in chat %d: %s", msg.Chat.ID, link.URL)

	status := h.sendStatus(msg)
	defer func() { h.deleteMessage(msg.Chat.ID, status) }()

	media, err := h.media.Fetch(ctx, link)
	if err != nil {
		log.Printf("fetch failed: %v", err)
		return
	}
	defer media.Release()

	h.deleteMessage(msg.Chat.ID, status)
	status = 0

	fileIDs, err := h.sendMedia(msg, media)
	if err != nil && media.Cached {
		log.Printf("cached send failed, downloading again: %v", err)
		media, err = h.media.FetchFresh(ctx, link)
		if err != nil {
			log.Printf("fetch failed: %v", err)
			return
		}
		defer media.Release()
		fileIDs, err = h.sendMedia(msg, media)
	}
	if err != nil {
		log.Printf("telegram send failed: %v", err)
		return
	}
	setFileIDs(&media, fileIDs)

	var sender domain.User
	if msg.From != nil {
		sender = toDomainUser(*msg.From)
	}
	if err := h.media.RecordDelivered(ctx, msg.Chat.ID, sender, link, media); err != nil {
		log.Printf("record download: %v", err)
	}
}

// allow applies per-user and per-chat rate limits, warning once per window.
func (h *Handler) allow(msg *tgbotapi.Message) bool {
	keys := []string{fmt.Sprintf("chat:%d", msg.Chat.ID)}
	limiters := []*usecase.RateLimiter{h.limits.PerChat}
	if msg.From != nil {
		keys = append(keys, fmt.Sprintf("user:%d", msg.From.ID))
		limiters = append(limiters, h.limits.PerUser)
	}

	for i, l := range limiters {
		ok, first := l.Allow(keys[i])
		if ok {
			continue
		}
		log.Printf("rate limited %s", keys[i])
		if first {
			h.reply(msg, rateLimitText)
		}
		return false
	}
	return true
}

func setFileIDs(m *domain.Media, fileIDs []string) {
	switch {
	case m.Video != nil && len(fileIDs) > 0:
		m.Video.FileID = fileIDs[0]
	case m.Post != nil:
		// A partial album must not be cached as the whole post.
		if len(fileIDs) == len(m.Post.Images) {
			m.Post.ImageFileIDs = fileIDs
		}
	}
}

func (h *Handler) reply(msg *tgbotapi.Message, text string) {
	cfg := tgbotapi.NewMessage(msg.Chat.ID, text)
	cfg.ReplyToMessageID = msg.MessageID
	cfg.AllowSendingWithoutReply = true
	if _, err := h.api.Send(cfg); err != nil {
		log.Printf("reply failed: %v", err)
	}
}

func (h *Handler) sendStatus(msg *tgbotapi.Message) int {
	cfg := tgbotapi.NewMessage(msg.Chat.ID, statusText)
	cfg.ReplyToMessageID = msg.MessageID
	cfg.AllowSendingWithoutReply = true
	sent, err := h.api.Send(cfg)
	if err != nil {
		log.Printf("status message failed: %v", err)
		return 0
	}
	return sent.MessageID
}

func (h *Handler) deleteMessage(chatID int64, messageID int) {
	if messageID == 0 {
		return
	}
	if _, err := h.api.Request(tgbotapi.NewDeleteMessage(chatID, messageID)); err != nil {
		log.Printf("delete message failed: %v", err)
	}
}

func toDomainChat(c *tgbotapi.Chat) domain.Chat {
	if c == nil {
		return domain.Chat{}
	}
	title := c.Title
	if title == "" {
		title = c.FirstName
		if c.LastName != "" {
			title += " " + c.LastName
		}
	}
	return domain.Chat{ID: c.ID, Type: c.Type, Title: title, Username: c.UserName}
}

func toDomainUser(u tgbotapi.User) domain.User {
	return domain.User{
		ID:        u.ID,
		Username:  u.UserName,
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

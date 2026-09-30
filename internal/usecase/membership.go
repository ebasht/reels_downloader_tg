package usecase

import (
	"context"
	"fmt"
	"sync"

	"video_download_bot/internal/domain"
)

type ChatRepository interface {
	// RecordMembership stores the bot's new status in the chat and who changed it.
	RecordMembership(ctx context.Context, change domain.MembershipChange) error
	// EnsureChat registers a chat the bot is in, if it isn't stored yet.
	EnsureChat(ctx context.Context, chat domain.Chat) error
}

type MembershipService struct {
	repo  ChatRepository
	known sync.Map // chat ID -> struct{}
}

func NewMembershipService(repo ChatRepository) *MembershipService {
	return &MembershipService{repo: repo}
}

func (s *MembershipService) HandleBotMembership(ctx context.Context, change domain.MembershipChange) error {
	if err := s.repo.RecordMembership(ctx, change); err != nil {
		return fmt.Errorf("record membership in chat %d: %w", change.Chat.ID, err)
	}
	if change.NewStatus.IsPresent() {
		s.known.Store(change.Chat.ID, struct{}{})
	} else {
		s.known.Delete(change.Chat.ID)
	}
	return nil
}

// TouchChat makes sure a chat the bot receives messages from is registered,
// covering chats the bot joined before it started tracking membership.
func (s *MembershipService) TouchChat(ctx context.Context, chat domain.Chat) error {
	if _, ok := s.known.Load(chat.ID); ok {
		return nil
	}
	if err := s.repo.EnsureChat(ctx, chat); err != nil {
		return fmt.Errorf("ensure chat %d: %w", chat.ID, err)
	}
	s.known.Store(chat.ID, struct{}{})
	return nil
}

package domain

import (
	"errors"
	"time"
)

// ErrMessageGone means a message can no longer be deleted: it is already
// gone, too old, or the bot left the chat. Retrying will not help.
var ErrMessageGone = errors.New("message can no longer be deleted")

// ScheduledDeletion is a bot message to delete from a chat at a given time.
type ScheduledDeletion struct {
	ChatID    int64
	MessageID int
	At        time.Time
}

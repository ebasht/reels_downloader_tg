package domain

import "time"

type User struct {
	ID        int64
	Username  string
	FirstName string
	LastName  string
}

type Chat struct {
	ID       int64
	Type     string
	Title    string
	Username string
}

// MemberStatus is the bot's status in a chat as reported by Telegram.
type MemberStatus string

const (
	StatusCreator       MemberStatus = "creator"
	StatusAdministrator MemberStatus = "administrator"
	StatusMember        MemberStatus = "member"
	StatusRestricted    MemberStatus = "restricted"
	StatusLeft          MemberStatus = "left"
	StatusKicked        MemberStatus = "kicked"
)

// IsPresent reports whether the bot is inside the chat with this status.
func (s MemberStatus) IsPresent() bool {
	switch s {
	case StatusCreator, StatusAdministrator, StatusMember, StatusRestricted:
		return true
	default:
		return false
	}
}

// MembershipChange describes the bot being added to or removed from a chat.
type MembershipChange struct {
	Chat      Chat
	Actor     User
	OldStatus MemberStatus
	NewStatus MemberStatus
	At        time.Time
}

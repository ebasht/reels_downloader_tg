package domain

// BotMessage is a chat message the bot sent and will delete later.
type BotMessage struct {
	ChatID    int64
	MessageID int
}

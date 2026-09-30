package telegram

import (
	"net/http"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Uploads of ~50MB videos over a slow link must fit in this timeout.
const httpTimeout = 5 * time.Minute

// NewBotAPI creates a Telegram client whose errors never contain the bot token.
// tgbotapi puts the token into request URLs, and net/http includes the URL in
// its errors, which end up in logs.
func NewBotAPI(token string) (*tgbotapi.BotAPI, error) {
	client := &redactingClient{
		inner: &http.Client{Timeout: httpTimeout},
		token: token,
	}
	return tgbotapi.NewBotAPIWithClient(token, tgbotapi.APIEndpoint, client)
}

type redactingClient struct {
	inner *http.Client
	token string
}

func (c *redactingClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.inner.Do(req)
	if err != nil {
		// Deliberately not wrapping: the original error would expose the token.
		return nil, redactedError(strings.ReplaceAll(err.Error(), c.token, "<redacted>"))
	}
	return resp, nil
}

type redactedError string

func (e redactedError) Error() string { return string(e) }

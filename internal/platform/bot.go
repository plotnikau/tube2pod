package platform

import (
	"time"

	tb "gopkg.in/tucnak/telebot.v2"
)

func NewTelegramBot(token string) (*tb.Bot, error) {
	return tb.NewBot(tb.Settings{
		Token:  token,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
}

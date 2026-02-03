package app

import (
	"context"
	tb "gopkg.in/tucnak/telebot.v2"
)

// Downloader defines the interface for downloading videos.
type Downloader interface {
	Download(ctx context.Context, url string) (success bool, title string, id string)
}

// Converter defines the interface for extracting audio from video files.
type Converter interface {
	ExtractAudio(fileId string) bool
}

// Bot defines the interface for interacting with the Telegram bot.
type Bot interface {
	Send(to tb.Recipient, what interface{}, options ...interface{}) (*tb.Message, error)
	Edit(msg tb.Editable, what interface{}, options ...interface{}) (*tb.Message, error)
	Delete(msg tb.Editable) error
}

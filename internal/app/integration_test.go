package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	tb "gopkg.in/tucnak/telebot.v2"
)

func TestFullWorkflow_Integration(t *testing.T) {
	mockDownloader := new(MockDownloader)
	mockConverter := new(MockConverter)
	mockBot := new(MockBot)

	processor := NewProcessor(mockDownloader, mockConverter, mockBot)

	sender := &tb.User{FirstName: "Test"}
	chat := &tb.Chat{ID: 123}
	message := &tb.Message{
		ID:     10,
		Text:   "https://youtube.com/watch?v=123",
		Sender: sender,
		Chat:   chat,
	}

	// 1. Download
	sentMsg1 := &tb.Message{ID: 100, Text: "*Download* ...", Chat: chat}
	mockBot.On("Send", sender, "*Download* ...", mock.Anything).Return(sentMsg1, nil)
	mockDownloader.On("Download", mock.Anything, message.Text).Return(true, "Title", "VideoID", "ThumbPath")
	mockBot.On("Delete", message).Return(nil)

	// 2. Convert & 3. Delivery (Multiple Edit calls)
	mockBot.On("Edit", mock.Anything, mock.Anything, mock.Anything).Return(func(msg tb.Editable, what interface{}, options ...interface{}) *tb.Message {
		return &tb.Message{ID: 100, Chat: chat, Text: what.(string)}
	}, nil)

	mockConverter.On("ExtractAudio", "VideoID").Return(true)
	mockBot.On("Delete", mock.MatchedBy(func(msg tb.Editable) bool {
		m, ok := msg.(*tb.Message)
		return ok && m.ID == 100
	})).Return(nil)

	// Start workers
	processor.StartWorkers(1, 1, 1)

	// Trigger the workflow
	processor.ProcessMessage(message)

	// Wait for completion (HandleUpload is the last step)
	// We'll wait until all expectations are met or timeout
	timeout := time.After(1 * time.Second)
	tick := time.Tick(10 * time.Millisecond)

	completed := false
	for {
		select {
		case <-timeout:
			t.Fatal("Timeout waiting for workflow completion")
		case <-tick:
			// Check if all expectations were met
			// This is a bit hacky but works for this case
			if len(mockConverter.Calls) > 0 {
				completed = true
			}
		}
		if completed {
			break
		}
	}

	// Give a little more time for the final cleanup
	time.Sleep(50 * time.Millisecond)

	mockBot.AssertExpectations(t)
	mockDownloader.AssertExpectations(t)
	mockConverter.AssertExpectations(t)
}

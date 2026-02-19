package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	tb "gopkg.in/tucnak/telebot.v2"
)

type MockDownloader struct {
	mock.Mock
}

func (m *MockDownloader) Download(ctx context.Context, url string) (bool, string, string, string) {
	args := m.Called(ctx, url)
	return args.Bool(0), args.String(1), args.String(2), args.String(3)
}

type MockConverter struct {
	mock.Mock
}

func (m *MockConverter) ExtractAudio(fileId string) bool {
	args := m.Called(fileId)
	return args.Bool(0)
}

type MockBot struct {
	mock.Mock
}

func (m *MockBot) Send(to tb.Recipient, what interface{}, options ...interface{}) (*tb.Message, error) {
	args := m.Called(to, what, options)
	msg, ok := args.Get(0).(*tb.Message)
	if !ok {
		if f, ok := args.Get(0).(func(tb.Recipient, interface{}, ...interface{}) *tb.Message); ok {
			msg = f(to, what, options...)
		}
	}
	return msg, args.Error(1)
}

func (m *MockBot) Edit(msg tb.Editable, what interface{}, options ...interface{}) (*tb.Message, error) {
	args := m.Called(msg, what, options)
	res, ok := args.Get(0).(*tb.Message)
	if !ok {
		if f, ok := args.Get(0).(func(tb.Editable, interface{}, ...interface{}) *tb.Message); ok {
			res = f(msg, what, options...)
		}
	}
	return res, args.Error(1)
}

func (m *MockBot) Delete(msg tb.Editable) error {
	args := m.Called(msg)
	return args.Error(0)
}

func TestProcessMessage_InvalidLink(t *testing.T) {
	mockBot := new(MockBot)
	processor := NewProcessor(nil, nil, mockBot)

	message := &tb.Message{
		Text:   "not a link",
		Sender: &tb.User{FirstName: "Test"},
	}

	mockBot.On("Send", message.Sender, mock.Anything, mock.Anything).Return(&tb.Message{}, nil)

	processor.ProcessMessage(message)

	mockBot.AssertExpectations(t)
}

func TestHandleDownload_Success(t *testing.T) {
	mockDownloader := new(MockDownloader)
	mockBot := new(MockBot)
	processor := NewProcessor(mockDownloader, nil, mockBot)

	sender := &tb.User{FirstName: "Test"}
	message := &tb.Message{Text: "https://youtube.com/watch?v=123", Sender: sender}
	task := DataEnvelope{URL: "https://youtube.com/watch?v=123", Message: message}

	sentMsg := &tb.Message{Text: "*Download* ..."}
	mockBot.On("Send", sender, "*Download* ...", mock.Anything).Return(sentMsg, nil)
	mockDownloader.On("Download", mock.Anything, task.URL).Return(true, "Title", "VideoID", "ThumbPath")
	mockBot.On("Delete", message).Return(nil)

	// We need to capture the output channel to prevent blocking
	go func() {
		processor.HandleDownload(task)
	}()

	receivedTask := <-processor.ConvertChan
	if receivedTask.VideoID != "VideoID" {
		t.Errorf("Expected VideoID, got %s", receivedTask.VideoID)
	}
	if receivedTask.Title != "Title" {
		t.Errorf("Expected Title, got %s", receivedTask.Title)
	}
	if receivedTask.ThumbnailPath != "ThumbPath" {
		t.Errorf("Expected ThumbPath, got %s", receivedTask.ThumbnailPath)
	}

	mockBot.AssertExpectations(t)
	mockDownloader.AssertExpectations(t)
}

func TestHandleConvert_Success(t *testing.T) {
	mockConverter := new(MockConverter)
	mockBot := new(MockBot)
	processor := NewProcessor(nil, mockConverter, mockBot)

	sentMsg := &tb.Message{
		Text: "*Download* ...",
		Chat: &tb.Chat{ID: 123},
	}
	task := DataEnvelope{VideoID: "VideoID", Title: "Title", Message: sentMsg}

	mockBot.On("Edit", mock.Anything, "*Download* ... *Extract audio* ... ", mock.Anything).Return(&tb.Message{Text: "*Download* ... *Extract audio* ... "}, nil)
	mockConverter.On("ExtractAudio", "VideoID").Return(true)

	go func() {
		processor.HandleConvert(task)
	}()

	receivedTask := <-processor.UploadChan
	if receivedTask.VideoID != "VideoID" {
		t.Errorf("Expected VideoID, got %s", receivedTask.VideoID)
	}

	mockBot.AssertExpectations(t)
	mockConverter.AssertExpectations(t)
}

func TestHandleUpload_Success(t *testing.T) {
	mockBot := new(MockBot)
	processor := NewProcessor(nil, nil, mockBot)

	chat := &tb.Chat{ID: 123}
	sentMsg := &tb.Message{
		ID:   456,
		Text: "*Download* ... *Extract audio* ... ",
		Chat: chat,
	}
	task := DataEnvelope{VideoID: "VideoID", Title: "Title", Message: sentMsg}

	// Mock SendAudio (it uses Glob, so we might need to handle that or mock it)
	// For now, let's assume it doesn't find any files in the test environment

	mockBot.On("Delete", mock.Anything).Return(nil)

	processor.HandleUpload(task)

	mockBot.AssertExpectations(t)
}

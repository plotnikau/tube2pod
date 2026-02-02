package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
)

type DataEnvelope struct {
	Message *tb.Message
	URL     string
	VideoID string
	Title   string
}

type Processor struct {
	downloader Downloader
	converter  Converter
	uploader   Uploader
	bot        Bot

	archiveUploadEnabled bool

	DownloadChan chan DataEnvelope
	ConvertChan  chan DataEnvelope
	UploadChan   chan DataEnvelope
}

func NewProcessor(downloader Downloader, converter Converter, uploader Uploader, bot Bot, archiveUploadEnabled bool) *Processor {
	return &Processor{
		downloader:           downloader,
		converter:            converter,
		uploader:             uploader,
		bot:                  bot,
		archiveUploadEnabled: archiveUploadEnabled,
		DownloadChan:         make(chan DataEnvelope),
		ConvertChan:          make(chan DataEnvelope),
		UploadChan:           make(chan DataEnvelope),
	}
}

func (p *Processor) StartWorkers(downloadWorkers, convertWorkers, uploadWorkers int) {
	for i := 0; i < downloadWorkers; i++ {
		go p.DownloadWorker()
	}
	for i := 0; i < convertWorkers; i++ {
		go p.ConvertWorker()
	}
	for i := 0; i < uploadWorkers; i++ {
		go p.UploadWorker()
	}
}

func (p *Processor) DownloadWorker() {
	log.Debug("Download Worker starting")

	for {
		task, ok := <-p.DownloadChan

		if !ok {
			log.Warn("problem with download channel!")
			return
		}
		p.HandleDownload(task)
	}
}

func (p *Processor) HandleDownload(task DataEnvelope) {
	url := task.URL
	message := task.Message

	log.Debug("[DOWNLOAD WORKER]", url)

	ts := time.Now()

	sentMessage, _ := p.SendMessage(message.Sender, "*Download* ...")
	success, title, id := p.downloader.Download(context.Background(), url)
	if !success {
		p.UpdateSentMessage(sentMessage, "\nError occurred")
		return
	}

	durationDl := time.Since(ts)
	log.Info("==> Successfully downloaded video: ", id)
	log.Info("==> Download took ", durationDl)

	p.bot.Delete(task.Message)

	task.Title = title
	task.VideoID = id
	task.Message = sentMessage

	p.ConvertChan <- task
}

func (p *Processor) ConvertWorker() {
	log.Debug("Convert Worker starting")
	for {
		task, ok := <-p.ConvertChan

		if !ok {
			log.Warn("problem with convert channel!")
			return
		}
		p.HandleConvert(task)
	}
}

func (p *Processor) HandleConvert(task DataEnvelope) {
	log.Debug("[CONVERT WORKER]", task.Title)

	sentMessage := task.Message
	id := task.VideoID

	sentMessage, _ = p.UpdateSentMessage(sentMessage, " *Extract audio* ... ")

	ts := time.Now()
	success := p.converter.ExtractAudio(id)

	if !success {
		p.UpdateSentMessage(sentMessage, "\nError occurred")
		return
	}
	durationConvert := time.Since(ts)
	log.Info("==> Successfully converted to audio: ", id)
	log.Info("==> Convert to mp3 took ", durationConvert)

	task.Message = sentMessage

	p.UploadChan <- task
}

func (p *Processor) UploadWorker() {
	log.Debug("Upload Worker starting")
	for {
		task, ok := <-p.UploadChan

		if !ok {
			log.Warn("problem with upload channel!")
			return
		}
		p.HandleUpload(task)
	}
}

func (p *Processor) HandleUpload(task DataEnvelope) {
	log.Debug("[UPLOAD WORKER] ", task.URL)

	sentMessage := task.Message
	id := task.VideoID
	title := task.Title

	p.SendAudio(id, title, sentMessage.Chat)

	if p.archiveUploadEnabled {
		archivePrefix := ArchiveItemPrefix + strconv.FormatInt(sentMessage.Chat.ID, 10) + "-"

		sentMessage, _ = p.UpdateSentMessage(sentMessage, " *Upload to podcast* ... ")
		ts := time.Now()
		success := p.uploader.UploadToArchive(id, title, archivePrefix)
		if !success {
			p.UpdateSentMessage(sentMessage, "Error")
			return
		}
		durationUpload := time.Since(ts)
		log.Info("==> Successfully uploaded to archive.org: ", title)
		log.Info("==> Upload to archive took ", durationUpload)

		playlistUrl := ArchiveSearchQueryUrl + archivePrefix + ArchiveSearchParams

		p.UpdateSentMessage(sentMessage, " *Done!*\n_It will take a couple of minutes to index a new file_\nAdd this [Link]("+playlistUrl+") to your podcast player")
	}

	p.Cleanup(id, sentMessage)
}

func (p *Processor) SendMessage(to tb.Recipient, text string) (*tb.Message, error) {
	return p.bot.Send(to, text, tb.ModeMarkdown)
}

func (p *Processor) UpdateSentMessage(sentMessage *tb.Message, text string) (*tb.Message, error) {
	s, i := sentMessage.MessageSig()
	storedMessage := tb.StoredMessage{MessageID: s, ChatID: i}
	return p.bot.Edit(storedMessage, sentMessage.Text+text, tb.ModeMarkdown)
}

func (p *Processor) Cleanup(fileId string, message *tb.Message) {
	filenames, _ := filepath.Glob(TmpDir + fileId + "*.mp3")
	for _, filename := range filenames {
		os.Remove(filename)
	}
	os.Remove(GetVideoFilename(fileId))
	p.bot.Delete(message)
}

func (p *Processor) SendAudio(id string, title string, chat *tb.Chat) {
	filenames, _ := filepath.Glob(TmpDir + id + ".0*.mp3")
	for i := len(filenames) - 1; i >= 0; i-- {
		filename := filenames[i]
		split := strings.Split(filename, ".")
		t := title + " : " + split[1]
		a := &tb.Audio{File: tb.FromDisk(filename), Title: t, Caption: t, FileName: t}
		_, err := p.bot.Send(chat, a)
		if err != nil {
			log.Error("Failed to send audio:", err)
			return
		}
	}
}

func (p *Processor) ProcessMessage(message *tb.Message) {
	text := message.Text

	if !strings.HasPrefix(text, "http") {
		p.SendMessage(message.Sender, "This seems not to be a valid link starting with https: "+text)
	} else {
		p.EnqueueYoutubeLink(text, message)
	}
}

func (p *Processor) EnqueueYoutubeLink(uri string, message *tb.Message) {
	task := DataEnvelope{URL: uri, Message: message}
	p.DownloadChan <- task
}

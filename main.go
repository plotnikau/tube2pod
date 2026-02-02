package main

import (
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"

	"main.go/internal/app"
	"main.go/internal/platform"
)

const (
	archiveAuthStringKey = "ARCHIVE_AUTH_STRING"
	telegramBotTokenKey  = "TELEGRAM_BOT_TOKEN"

	DOWNLOAD_WORKERS = 5
	CONVERT_WORKERS  = 2
	UPLOAD_WORKERS   = 5
)

func main() {
	archiveAuthString := os.Getenv(archiveAuthStringKey)
	telegramBotToken := os.Getenv(telegramBotTokenKey)

	archiveUploadEnabled := true
	if archiveAuthString == "" {
		archiveUploadEnabled = false
		log.Info("Env variable ARCHIVE_AUTH_STRING is missing, no upload to internet archieve will be done, just delivery to telegram. \nIf you want to populate podcast playlist with audio, look here for detailed information: https://archive.org/services/docs/api/ias3.html")
	}

	if telegramBotToken == "" {
		log.Error("Env variable TELEGRAM_BOT_TOKEN is missing. \nAsk BotFather to create bot for you: https://telegram.me/BotFather")
		return
	}

	log.Info("Starting everything...")
	prepareInfra()

	bot, err := platform.NewTelegramBot(telegramBotToken)
	if err != nil {
		log.Fatal(err)
	}

	downloader := &platform.YoutubeDownloader{}
	converter := &platform.FfmpegConverter{}
	uploader := &platform.ArchiveUploader{AuthString: archiveAuthString}

	processor := app.NewProcessor(downloader, converter, uploader, bot, archiveUploadEnabled)

	setupHandlers(bot, processor)

	processor.StartWorkers(DOWNLOAD_WORKERS, CONVERT_WORKERS, UPLOAD_WORKERS)

	bot.Start()
}

func prepareInfra() {
	// temporary dir
	os.MkdirAll(app.TmpDir, os.ModePerm)

	// log levels & standard output
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

func setupHandlers(bot *tb.Bot, processor *app.Processor) {
	bot.Handle("/start", func(m *tb.Message) {
		const usageInfo = "This bot creates your personal podcast from videos selected by you. \nSend youtube video links to tube2pod bot and it will create your own youtube audio podcast-feed."
		text := "Hello, " + m.Sender.FirstName + "\n" + usageInfo
		bot.Send(m.Sender, text)
	})

	bot.Handle("/cleanup", func(m *tb.Message) {
		files, err := filepath.Glob(app.TmpDir + "*")
		if err != nil {
			bot.Send(m.Sender, "Error while cleaning up: "+err.Error())
			return
		}

		count := 0
		for _, file := range files {
			if err := os.Remove(file); err != nil {
				log.Error("Failed to remove file:", file, err)
				continue
			}
			count++
		}

		bot.Send(m.Sender, fmt.Sprintf("Cleaned up %d temporary files", count))
	})

	bot.Handle(tb.OnText, func(m *tb.Message) {
		go processor.ProcessMessage(m)
	})
}

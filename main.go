package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/wader/goutubedl"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	tb "gopkg.in/tucnak/telebot.v2"
)

const (
	tmpDir   = "./tmp/"
	extAudio = ".mp3"
	extVideo = ".mp4"
)

const (
	archiveBaseUrl        = "http://s3.us.archive.org/"
	archiveItemPrefix     = "youtube-audio-"
	archiveSearchQueryUrl = "https://archive.org/advancedsearch.php?q="
	archiveSearchParams   = "&rows=100&page=1&callback=callback&save=yes&output=rss"
	archiveAuthStringKey  = "ARCHIVE_AUTH_STRING"
	telegramBotTokenKey   = "TELEGRAM_BOT_TOKEN"
	empty                 = ""
)

var (
	telegramBotToken     = ""
	archiveAuthString    = ""
	archiveUploadEnabled = false
)

// used to pass one job from worker to worker via go channels
type dataEnvelope struct {
	message *tb.Message
	url     string
	videoId string
	title   string
}

// number of parallel running workers for video download, converting 2 audio and upload to archive.org
// - converter is the most resource consuming (uses ffmpeg),
// - upload to archive.org is slowest (due to archive.org bandwidth limitation)
const (
	DOWNLOAD_WORKERS = 5
	CONVERT_WORKERS  = 2
	UPLOAD_WORKERS   = 5
)

var downloadChan = make(chan dataEnvelope)
var convertChan = make(chan dataEnvelope)
var uploadChan = make(chan dataEnvelope)

var bot *tb.Bot

func main() {
	// for test purposes
	//_, _, id := downloadVideo("https://www.youtube.com/watch?v=zdjQpqmvqtI")
	//extractAudio(id)

	do()
}

func do() {
	// archive.org API key and telegram bot token are read from environment variables
	// both are expected as mandatory
	archiveAuthString = os.Getenv(archiveAuthStringKey)
	telegramBotToken = os.Getenv(telegramBotTokenKey)

	if archiveAuthString == "" {
		archiveUploadEnabled = false
		log.Info("Env variable ARCHIVE_AUTH_STRING is missing, no upload to internet archieve will be done, just delivery to telegram. \nIf you want to populate podcast playlist with audio, look here for detailed information: https://archive.org/services/docs/api/ias3.html")
	} else {
		archiveUploadEnabled = true
	}

	if telegramBotToken == "" {
		log.Error("Env variable TELEGRAM_BOT_TOKEN is missing. \nAsk BotFather to create bot for you: https://telegram.me/BotFather")
		return
	}

	log.Info("Starting everything...")
	prepareInfra()

	bot, _ = setupTelegramBot(telegramBotToken)

	setupWorker()

	bot.Start()
}

func prepareInfra() {
	// temporary dir
	os.MkdirAll(tmpDir, os.ModePerm)

	// log levels & standard output
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

// Bot logic
func setupTelegramBot(botToken string) (*tb.Bot, error) {
	b, err := tb.NewBot(tb.Settings{
		Token:  botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return b, err
	}

	b.Handle("/start", func(m *tb.Message) {
		const usageInfo = "This bot creates your personal podcast from videos selected by you. \nSend youtube video links to tube2pod bot and it will create your own youtube audio podcast-feed."
		text := "Hello, " + m.Sender.FirstName + "\n" + usageInfo
		b.Send(m.Sender, text)
	})

	b.Handle("/cleanup", func(m *tb.Message) {
		files, err := filepath.Glob(tmpDir + "*")
		if err != nil {
			b.Send(m.Sender, "Error while cleaning up: "+err.Error())
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

		b.Send(m.Sender, fmt.Sprintf("Cleaned up %d temporary files", count))
	})

	b.Handle(tb.OnText, func(m *tb.Message) {
		go processMessage(m)
	})

	return b, err
}

func setupWorker() {
	// video download, audio extraction and audio upload "stages" are running in separate goroutines, multiple workers per stage
	// TODO: find out the best fit
	for i := 0; i < DOWNLOAD_WORKERS; i++ {
		go downloadWorker()
	}
	for i := 0; i < CONVERT_WORKERS; i++ {
		go convertWorker()
	}
	for i := 0; i < UPLOAD_WORKERS; i++ {
		go uploadWorker()
	}
}

func downloadWorker() {
	log.Debug("Download Worker starting")

	for {
		task, ok := <-downloadChan

		if !ok {
			log.Warn("problem with download channel!")
			continue
		}

		url := task.url
		message := task.message

		log.Debug("[DOWNLOAD WORKER]", url)

		// do work

		// download from youtube
		ts := time.Now()

		sentMessage, _ := sendMessage(message.Sender, "*Download* ...")
		success, title, id := downloadVideo(url)
		if !success {
			updateSentMessage(sentMessage, "\nError occurred")
			continue
		}

		durationDl := time.Since(ts)
		log.Info("==> Successfully downloaded video: ", id)
		log.Info("==> Download took ", durationDl)

		bot.Delete(task.message)

		// prepare and trigger audio extraction (put task to convert channel)
		task.title = title
		task.videoId = id
		task.message = sentMessage

		convertChan <- task
	}
}

func convertWorker() {
	log.Debug("Convert Worker starting")
	for {
		task, ok := <-convertChan

		if !ok {
			log.Warn("problem with download channel!")
			continue
		}

		log.Debug("[CONVERT WORKER]", task.title)

		// do work
		sentMessage := task.message
		id := task.videoId

		// convert to mp3
		sentMessage, _ = updateSentMessage(sentMessage, " *Extract audio* ... ")

		ts := time.Now()
		success := extractAudio(id)

		if !success {
			updateSentMessage(sentMessage, "\nError occurred")
			continue
		}
		durationConvert := time.Since(ts)
		log.Info("==> Successfully converted to audio: ", id)
		log.Info("==> Convert to mp3 took ", durationConvert)

		task.message = sentMessage

		uploadChan <- task
	}
}

func uploadWorker() {
	log.Debug("Upload Worker starting")
	for {
		task, ok := <-uploadChan

		if !ok {
			log.Warn("problem with download channel!")
			continue
		}

		log.Debug("[UPLOAD WORKER] ", task.url)

		// do work

		// to have separate "playlists" for every telegram user: ChatID is used as unique part of archive item prefix
		sentMessage := task.message
		id := task.videoId
		title := task.title

		// send to telegram
		sendAudio(id, title, sentMessage.Chat)

		if archiveUploadEnabled {
			// this archivePrefix will be also used to generate playlist link
			archivePrefix := archiveItemPrefix + strconv.FormatInt(sentMessage.Chat.ID, 10) + "-"

			// upload to archive.org
			sentMessage, _ = updateSentMessage(sentMessage, " *Upload to podcast* ... ")
			ts := time.Now()
			success := uploadToArchive(id, title, archivePrefix)
			if !success {
				updateSentMessage(sentMessage, "Error")
				continue
			}
			durationUpload := time.Since(ts)
			log.Info("==> Successfully uploaded to archive.org: ", title)
			log.Info("==> Upload to archive took ", durationUpload)

			playlistUrl := archiveSearchQueryUrl + archivePrefix + archiveSearchParams

			updateSentMessage(sentMessage, " *Done!*\n_It will take a couple of minutes to index a new file_\nAdd this [Link]("+playlistUrl+") to your podcast player")
		}

		cleanup(id, sentMessage)
	}

}

func processMessage(message *tb.Message) {
	text := message.Text

	// TODO: verify that we have real youtube url, for now just do a dummy http(s) prefix check
	if !strings.HasPrefix(text, "http") {
		sendMessage(message.Sender, "This seems not to be a valid link starting with https: "+text)
	} else {
		enqueueYoutubeLink(text, message)
	}

}

func enqueueYoutubeLink(uri string, message *tb.Message) {
	task := dataEnvelope{url: uri, message: message}
	downloadChan <- task
}

func sendMessage(toUser *tb.User, text string) (*tb.Message, error) {
	message, e := bot.Send(toUser, text, tb.ModeMarkdown)
	return message, e
}

func updateSentMessage(sentMessage *tb.Message, text string) (*tb.Message, error) {
	s, i := sentMessage.MessageSig()
	storedMessage := tb.StoredMessage{MessageID: s, ChatID: i}
	sentMessage, err := bot.Edit(storedMessage, sentMessage.Text+text, tb.ModeMarkdown)
	return sentMessage, err
}

func cleanup(fileId string, message *tb.Message) {
	filenames, _ := filepath.Glob(tmpDir + fileId + "*.mp3")
	for _, filename := range filenames {
		os.Remove(filename)
	}
	os.Remove(getVideoFilename(fileId))
	bot.Delete(message)
}

func downloadVideo(url string) (success bool, title string, id string) {

	//client := youtube.Client{
	//	HTTPClient: http.DefaultClient,
	//}

	//dl := downloader.Downloader{OutputDir: tmpDir, Client: client}

	log.Info("starting download")

	result, err := goutubedl.New(context.Background(), url, goutubedl.Options{})
	if err != nil {
		log.Error("Failed to get video info", err)
		return false, empty, empty
	}

	title = result.Info.Title
	log.Debug("==> Downloading video: ", title)
	fileId := result.Info.ID
	filename := getVideoFilename(fileId)

	downloadResult, err := result.Download(context.Background(), "best")
	if err != nil {
		log.Error("Failed to download video", err)
		return false, empty, empty
	}
	defer downloadResult.Close()
	f, err := os.Create(filename)
	if err != nil {
		log.Error("Failed to save downloaded video to file")
		return false, empty, empty
	}
	defer f.Close()
	io.Copy(f, downloadResult)

	log.Debug("==> Video done")

	return true, title, fileId
}

func extractAudio(fileId string) (success bool) {

	// ffmpeg -y -loglevel quiet -i video.mp4 -vn audio.mp3
	fnVideo := getVideoFilename(fileId)
	fnAudio := getAudioFilename(fileId)

	log.Debug("convert " + fnVideo + " -> " + fnAudio)

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Error("ffmpeg not found")
		return false
	} else {
		log.Debug("==> Extracting audio...")
		cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", fnVideo, "-vn", fnAudio)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Error("Failed to extract audio:", err)
			return false
		} else {
			fnSegments := getAudioFilenamePattern(fileId)
			cmd = exec.Command(ffmpeg, "-y", "-i", fnAudio, "-loglevel", "quiet", "-c", "copy", "-map", "0", "-segment_time", "00:45:00", "-f", "segment", "-vn", fnSegments)
			if err = cmd.Run(); err != nil {
				log.Error("Failed to extract audio:", err)
				return false
			} else {
				log.Debug("==> Audio done")
				return true
			}
		}
	}

}

func uploadToArchive(fileId string, title string, prefix string) (success bool) {
	if !archiveUploadEnabled {
		return false
	}
	filename := getAudioFilename(fileId)

	itemId := uuid.New()
	url := archiveBaseUrl + prefix + itemId.String() + "/" + "audio" + extAudio
	log.Debug("uploading "+filename+" to ", url)

	file, err := os.Open(filename)
	if err != nil {
		log.Error(err)
		return false
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, _ := writer.CreateFormFile("file", filepath.Base(file.Name()))
	io.Copy(part, file)
	writer.Close()

	req, _ := http.NewRequest(http.MethodPut, url, body)
	req.Header.Add("Content-Type", writer.FormDataContentType())
	req.Header.Add("X-Amz-Auto-Make-Bucket", "1")
	req.Header.Add("X-Archive-Meta-Mediatype", "audio")
	req.Header.Add("X-Archive-Meta-Title", title)
	authString := "LOW " + archiveAuthString
	req.Header.Add("Authorization", authString)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return false
	}

	defer res.Body.Close()
	resBody, _ := ioutil.ReadAll(res.Body)

	log.Debug(res.StatusCode)
	log.Debug(string(resBody))

	return true

}

func sendAudio(id string, title string, chat *tb.Chat) {
	filenames, _ := filepath.Glob(tmpDir + id + ".0*.mp3")
	for i := len(filenames) - 1; i >= 0; i-- {
		filename := filenames[i]
		//for _, filename := range filenames {
		split := strings.Split(filename, ".")
		//filename := getAudioFilename(id)
		t := title + " : " + split[1]
		a := &tb.Audio{File: tb.FromDisk(filename), Title: t, Caption: t, FileName: t}
		_, err := bot.Send(chat, a)
		if err != nil {
			log.Error("Failed to send audio:", err)
			return
		}
	}
}

func getAudioFilename(id string) string {
	return tmpDir + id + extAudio
}

func getAudioFilenamePattern(id string) string {
	return tmpDir + id + ".%02d" + extAudio
}

func getVideoFilename(id string) string {
	return tmpDir + id + extVideo
}

package main

import (
	"bytes"
	"github.com/rylio/ytdl"
	"github.com/satori/go.uuid"
	log "github.com/sirupsen/logrus"
	tb "gopkg.in/tucnak/telebot.v2"
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
)

const (
	tmpDir   = "./tmp/"
	extAudio = ".mp3"
	extVideo = ".mp4"
)

const (
	archiveBaseUrl       = "http://s3.us.archive.org/"
	archiveItemPrefix    = "youtube-audio-"
	archiveAuthStringKey = "ARCHIVE_AUTH_STRING"
	telegramBotTokenKey  = "TELEGRAM_BOT_TOKEN"
	empty                = ""
)

var (
	telegramBotToken  = ""
	archiveAuthString = ""
)

func main() {

	archiveAuthString = os.Getenv(archiveAuthStringKey)
	telegramBotToken = os.Getenv(telegramBotTokenKey)

	if archiveAuthString == "" {
		log.Error("Env variable ARCHIVE_AUTH_STRING is missing")
		return
	}
	if telegramBotToken == "" {
		log.Error("Env variables TELEGRAM_BOT_TOKEN is missing")
		return
	}

	prepareInfra()

	setupTelegramBot(telegramBotToken)
}

func prepareInfra() {
	// temporary dir
	os.MkdirAll(tmpDir, os.ModePerm)

	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
}

//
// Bot logic
//
func setupTelegramBot(botToken string) (*tb.Bot, error) {
	b, err := tb.NewBot(tb.Settings{
		Token:  botToken,
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})

	if err != nil {
		log.Fatal(err)
		return b, err
	}

	b.Handle("/hello", func(m *tb.Message) {
		b.Send(m.Sender, "hello world")
	})

	b.Handle(tb.OnText, func(m *tb.Message) {
		go processMessage(b, m)
	})

	b.Start()

	return b, err
}

func processMessage(bot *tb.Bot, message *tb.Message) {
	text := message.Text

	if !strings.HasPrefix(text, "http") {
		bot.Send(message.Sender, text)
	} else {
		_, id := processYoutubeLink(text, bot, message)
		cleanup(id)
		bot.Send(message.Sender, "==> Done!")
	}

}

//
// Youtube processing: download, convert to mp3 & upload to archive
//
func processYoutubeLink(url string, bot *tb.Bot, message *tb.Message) (success bool, fileId string) {

	archivePrefix := archiveItemPrefix + strconv.FormatInt(message.Chat.ID, 10) + "-"

	// download from youtube
	bot.Send(message.Sender, "Download...")
	ts := time.Now()
	success, title, id := downloadVideo(url)
	if !success {
		bot.Send(message.Sender, "Error occurred during download :(")
		return false, id
	}

	durationDl := time.Since(ts)
	log.Info("==> Successfully downloaded video: ", id)
	log.Info("==> Download took ", durationDl)

	// convert to mp3
	bot.Send(message.Sender, "Extract audio...")
	ts = time.Now()
	success = extractAudio(id)

	if !success {
		bot.Send(message.Sender, "Error occurred during extracting audio :(")
		return false, id
	}
	durationConvert := time.Since(ts)
	log.Info("==> Successfully converted to audio: ", id)
	log.Info("==> Convert to mp3 took ", durationConvert)

	// upload to archive.org
	bot.Send(message.Sender, "Upload to archive...")
	ts = time.Now()
	success = uploadToArchive(id, title, archivePrefix)
	if !success {
		bot.Send(message.Sender, "Error occurred during upload to archive :(")
		return false, id
	}
	durationUpload := time.Since(ts)
	log.Info("==> Successfully uploaded to archive.org: ", title)
	log.Info("==> Upload to archive took ", durationUpload)

	bot.Send(message.Sender, "==> Your archive prefix: "+archivePrefix)
	return true, id
}

func cleanup(fileId string) {
	os.Remove(getAudioFilename(fileId))
	os.Remove(getVideoFilename(fileId))
}

func downloadVideo(url string) (success bool, title string, id string) {

	vid, err := ytdl.GetVideoInfo(url)
	if err != nil {
		log.Error("Failed to get video info")
		return false, empty, empty
	}

	log.Info("==> Downloading video: ", vid.Title)
	fmtBestAudio := vid.Formats.Best(ytdl.FormatAudioBitrateKey)

	//filename := tmpDir + vid.Title + extVideo
	fileId := vid.ID
	filename := tmpDir + fileId + extVideo
	file, _ := os.Create(filename)
	defer file.Close()
	vid.Download(fmtBestAudio[0], file)

	log.Info("==> Video done")

	return true, vid.Title, fileId
}

func extractAudio(fileId string) (success bool) {

	// ffmpeg -y -loglevel quiet -i video.mp4 -vn audio.mp3
	fnVideo := getVideoFilename(fileId)
	fnAudio := getAudioFilename(fileId)

	log.Debug(fnVideo + " -> " + fnAudio)

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Error("ffmpeg not found")
		return false
	} else {
		log.Info("==> Extracting audio...")
		cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", fnVideo, "-vn", fnAudio)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			log.Error("Failed to extract audio:", err)
			return false
		} else {
			log.Info()
			log.Info("==> Audio done")
			return true
		}
	}

}

func uploadToArchive(fileId string, title string, prefix string) (success bool) {

	filename := getAudioFilename(fileId)

	itemId := uuid.NewV4()
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
	log.Debug(authString)
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

func getAudioFilename(id string) string {
	return tmpDir + id + extAudio
}

func getVideoFilename(id string) string {
	return tmpDir + id + extVideo
}

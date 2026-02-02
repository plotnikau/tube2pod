package platform

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/plotnikau/tube2pod/internal/app"
)

type ArchiveUploader struct {
	AuthString string
}

func (a *ArchiveUploader) UploadToArchive(fileId string, title string, prefix string) bool {
	filename := app.GetAudioFilename(fileId)

	itemId := uuid.New()
	url := app.ArchiveBaseUrl + prefix + itemId.String() + "/" + "audio" + app.ExtAudio
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
	authString := "LOW " + a.AuthString
	req.Header.Add("Authorization", authString)

	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		log.Error(err)
		return false
	}

	defer res.Body.Close()
	resBody, _ := io.ReadAll(res.Body)

	log.Debug(res.StatusCode)
	log.Debug(string(resBody))

	return true
}

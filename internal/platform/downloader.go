package platform

import (
	"context"
	"io"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/wader/goutubedl"
	"main.go/internal/app"
)

type YoutubeDownloader struct{}

func (y *YoutubeDownloader) Download(ctx context.Context, url string) (success bool, title string, id string) {
	log.Info("starting download")

	result, err := goutubedl.New(ctx, url, goutubedl.Options{})
	if err != nil {
		log.Error("Failed to get video info", err)
		return false, app.Empty, app.Empty
	}

	title = result.Info.Title
	log.Debug("==> Downloading video: ", title)
	fileId := result.Info.ID
	filename := app.GetVideoFilename(fileId)

	downloadResult, err := result.Download(ctx, "bestaudio")
	if err != nil {
		log.Error("Failed to download video", err)
		return false, app.Empty, app.Empty
	}
	defer downloadResult.Close()
	f, err := os.Create(filename)
	if err != nil {
		log.Error("Failed to save downloaded video to file")
		return false, app.Empty, app.Empty
	}
	defer f.Close()
	io.Copy(f, downloadResult)

	log.Debug("==> Video done")

	return true, title, fileId
}

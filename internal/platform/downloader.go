package platform

import (
	"context"
	"io"
	"net/http"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/wader/goutubedl"
	"main.go/internal/app"
)

type YoutubeDownloader struct{}

func (y *YoutubeDownloader) Download(ctx context.Context, url string) (success bool, title string, id string, thumb string) {
	log.Info("starting download")

	result, err := goutubedl.New(ctx, url, goutubedl.Options{})
	if err != nil {
		log.Error("Failed to get video info", err)
		return false, app.Empty, app.Empty, app.Empty
	}

	title = result.Info.Title
	log.Debug("==> Downloading video: ", title)
	fileId := result.Info.ID
	filename := app.GetVideoFilename(fileId)

	// Download thumbnail
	thumb = app.Empty
	if result.Info.Thumbnail != "" {
		thumb = y.downloadAndResizeThumbnail(result.Info.Thumbnail, fileId)
	}

	downloadResult, err := result.Download(ctx, "bestaudio")
	if err != nil {
		log.Error("Failed to download video", err)
		return false, app.Empty, app.Empty, app.Empty
	}
	defer downloadResult.Close()
	f, err := os.Create(filename)
	if err != nil {
		log.Error("Failed to save downloaded video to file")
		return false, app.Empty, app.Empty, app.Empty
	}
	defer f.Close()
	io.Copy(f, downloadResult)

	log.Debug("==> Video done")

	return true, title, fileId, thumb
}

func (y *YoutubeDownloader) downloadAndResizeThumbnail(url string, id string) string {
	log.Debug("==> Downloading thumbnail: ", url)
	resp, err := http.Get(url)
	if err != nil {
		log.Error("Failed to download thumbnail: ", err)
		return app.Empty
	}
	defer resp.Body.Close()

	tempThumb := app.TmpDir + id + "_raw"
	f, err := os.Create(tempThumb)
	if err != nil {
		log.Error("Failed to create temp thumbnail file: ", err)
		return app.Empty
	}
	defer os.Remove(tempThumb)

	_, err = io.Copy(f, resp.Body)
	f.Close()
	if err != nil {
		log.Error("Failed to save temp thumbnail: ", err)
		return app.Empty
	}

	thumbPath := app.GetThumbnailFilename(id)
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Error("ffmpeg not found for thumbnail resizing")
		return app.Empty
	}

	// Resize to 320x320 max while keeping aspect ratio, and convert to jpg
	cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", tempThumb, "-vf", "scale='if(gt(a,1),320,-1)':'if(gt(a,1),-1,320)'", thumbPath)
	if err := cmd.Run(); err != nil {
		log.Error("Failed to resize thumbnail with ffmpeg: ", err)
		return app.Empty
	}

	return thumbPath
}

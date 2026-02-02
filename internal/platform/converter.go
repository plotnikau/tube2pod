package platform

import (
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
	"github.com/plotnikau/tube2pod/internal/app"
)

type FfmpegConverter struct{}

func (f *FfmpegConverter) ExtractAudio(fileId string) bool {
	fnVideo := app.GetVideoFilename(fileId)
	fnAudio := app.GetAudioFilename(fileId)

	log.Debug("convert " + fnVideo + " -> " + fnAudio)

	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		log.Error("ffmpeg not found")
		return false
	}

	log.Debug("==> Extracting audio...")
	cmd := exec.Command(ffmpeg, "-y", "-loglevel", "quiet", "-i", fnVideo, "-vn", fnAudio)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Error("Failed to extract audio:", err)
		return false
	}

	fnSegments := app.GetAudioFilenamePattern(fileId)
	cmd = exec.Command(ffmpeg, "-y", "-i", fnAudio, "-loglevel", "quiet", "-c", "copy", "-map", "0", "-segment_time", "00:45:00", "-f", "segment", "-vn", fnSegments)
	if err := cmd.Run(); err != nil {
		log.Error("Failed to extract audio:", err)
		return false
	}

	log.Debug("==> Audio done")
	return true
}

# Agent Instructions

Telegram bot that converts YouTube videos into audio files and sends them via Telegram.

## Commands

```bash
go build -o tube2pod .
go test -v ./...
TELEGRAM_BOT_TOKEN="token" go run main.go
```

No linter or formatter is configured. CI only runs `go build` and `go test`. `go vet` is available for quick static checks.

## Architecture

3-stage concurrent pipeline connected by unbuffered Go channels:

```
ProcessMessage -> DownloadChan (5 workers) -> ConvertChan (2 workers) -> UploadChan (5 workers)
```

- **`internal/app`** — platform-agnostic interfaces (`Downloader`, `Converter`, `Bot`) and orchestration (`processor.go`). This is where business logic lives.
- **`internal/platform`** — concrete implementations: `yt-dlp` via `goutubedl` for downloads, `ffmpeg` for audio extraction/segmentation, `telebot.v2` for Telegram.
- **`main.go`** — wiring, env vars, worker count config, handler registration.

## Gotchas

- **Module name is `main.go`** — `go.mod` declares `module main.go`, so imports look like `"main.go/internal/app"`. Unconventional but intentional; do not rename.
- **System deps required**: `ffmpeg` and `yt-dlp` must be in PATH.
- **SponsorBlock**: In Docker, a wrapper script at `/usr/local/bin/yt-dlp` injects `--sponsorblock-remove sponsor` automatically. Locally, no SponsorBlock removal happens.
- **Audio files named `.mp4`** despite being audio-only (`bestaudio` format). The extension is misleading — this is by design for compatibility.
- **Segmentation**: Audio is split into 45-min chunks (`-segment_time 00:45:00`) to stay under Telegram's 50MB bot file limit.
- **No graceful shutdown** — workers are bare goroutine loops with no context cancellation.
- **`DataEnvelope.Message` is reassigned** during the pipeline (user message → sent status message). This is fragile; be careful modifying the struct.
- **`docker/Dockerfile.local` is broken** — references deprecated `Gopkg.toml`/`Gopkg.lock`. Do not use it. The main `Dockerfile` expects a pre-built binary.
- **No automatic cleanup** — temp files in `./tmp/` accumulate. Use `/cleanup` command or manage externally.
- **Temp file naming**: `{videoId}.mp4` (download), `{videoId}.mp3` (full audio), `{videoId}.00.mp3`/`{videoId}.01.mp3` etc. (segments), `{videoId}.jpg` (thumbnail). Pattern helpers are in `internal/app/utils.go`.
- **No retry/error propagation** — failed tasks are logged and silently dropped. No user notification beyond a generic error message.

## Testing

Mocks for all three interfaces are in `processor_test.go` using `testify/mock`. Tests are white-box (same `app` package). The integration test in `integration_test.go` starts real workers with mocks end-to-end.

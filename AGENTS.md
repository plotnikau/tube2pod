# Information for Coding Agents

Welcome, fellow agent! This project is a Telegram bot that converts YouTube videos into audio podcasts.

## Architecture Overview
- **`main.go`**: Entry point. Sets up environment variables, initializes the bot, and starts the processor workers.
- **`internal/app`**: Core logic.
    - `processor.go`: Orchestrates the download -> convert -> deliver pipeline using Go channels and workers.
    - `interfaces.go`: Defines the `Downloader`, `Converter`, and `Bot` interfaces to decouple logic from platforms.
    - `constants.go` & `utils.go`: Configuration and helper functions.
- **`internal/platform`**: Implementations of the interfaces.
    - `downloader.go`: Uses `yt-dlp` (via `goutubedl`) to download videos.
    - `converter.go`: Uses `ffmpeg` to extract and segment audio.
    - `bot.go`: Telegram bot implementation using `telebot.v2`.

## Key Workflows
1. **Message Received**: `processor.ProcessMessage` is called.
2. **Download**: Task added to `DownloadChan`. `DownloadWorker` uses `YoutubeDownloader`.
3. **Convert**: Task added to `ConvertChan`. `ConvertWorker` uses `FfmpegConverter`.
4. **Deliver**: Task added to `UploadChan`. `UploadWorker` sends audio files to Telegram and cleans up.

## Development & Testing
- **Testing**: Use `go test -v ./...`. We use `testify/mock` for mocking dependencies.
- **Dependencies**:
    - `ffmpeg` must be in the system PATH.
    - `yt-dlp` must be in the system PATH.
- **Environment Variables**:
    - `TELEGRAM_BOT_TOKEN`: Required for the bot to start.

## Recent Changes
- Removed Archive.org integration (it was deemed obsolete). The bot now focuses purely on Telegram delivery.
- Updated documentation and added this `AGENTS.md` file.

## Tech Stack
- Go 1.24
- [telebot.v2](https://github.com/tucnak/telebot)
- [goutubedl](https://github.com/wader/goutubedl) (wrapper for yt-dlp)
- ffmpeg for audio processing

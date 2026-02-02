# tube2pod :: podcast from youtube

![Go](https://github.com/plotnikau/tube2pod/workflows/Go/badge.svg?branch=master)

`tube2pod` is a Telegram bot that converts YouTube videos into a personal podcast feed. Simply send a YouTube link to the bot, and it will download the video, extract the audio, and optionally upload it to Internet Archive to generate a podcast-compatible RSS feed.

## Features

- **YouTube to MP3**: Automatically downloads and extracts audio from YouTube links.
- **Audio Splitting**: Large audio files are automatically split into manageable segments (45 minutes each).
- **Telegram Delivery**: Sends the converted audio files directly to you in Telegram.
- **Podcast Feed (Optional)**: Uploads audio to Internet Archive and provides a link to your personal podcast feed.
- **Concurrent Processing**: Multiple workers handle downloads, conversions, and uploads in parallel.

## Prerequisites

- **Go**: 1.22 or higher.
- **FFmpeg**: Required on the system for audio extraction and processing.

## Configuration

The application is configured via environment variables:

| Variable | Description | Required |
| --- | --- | --- |
| `TELEGRAM_BOT_TOKEN` | Your Telegram Bot API token (from @BotFather). | Yes |
| `ARCHIVE_AUTH_STRING` | Internet Archive S3 API keys (`access_key:secret_key`). | No |

If `ARCHIVE_AUTH_STRING` is not provided, the bot will still function and deliver audio via Telegram, but it will not upload to Internet Archive or generate a podcast feed.

## Installation & Usage

1. **Clone the repository**:
   ```bash
   git clone https://github.com/plotnikau/tube2pod.git
   cd tube2pod
   ```

2. **Build the application**:
   ```bash
   go build -o tube2pod .
   ```

3. **Run the bot**:
   ```bash
   export TELEGRAM_BOT_TOKEN="your_token_here"
   export ARCHIVE_AUTH_STRING="your_ia_keys_here" # Optional
   ./tube2pod
   ```

## Testing

The project includes a suite of unit and integration tests.

Run all tests:
```bash
go test -v ./...
```

## Project Structure

- `internal/app/`: Contains the core business logic, including the `Processor` and worker definitions.
- `internal/platform/`: Contains platform-specific implementations for external services (Telegram, YouTube, FFmpeg, Archive.org).
- `main.go`: The application entry point where dependencies are initialized and injected.

## License

This project is licensed under the MIT License - see the [LICENSE.txt](LICENSE.txt) file for details.

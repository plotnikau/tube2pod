# tube2pod :: YouTube to Telegram Audio Bot

![Go](https://github.com/plotnikau/tube2pod/workflows/Go/badge.svg?branch=master)

Hey there! **tube2pod** is a friendly Telegram bot that turns YouTube videos into audio files and sends them straight to your chat. It's perfect for creating your own "podcast" from YouTube content you'd rather listen to than watch.

## ✨ Features
- **Easy Peasy**: Just send a YouTube link, and the bot does the rest.
- **Auto-Split**: Large videos are automatically split into 45-minute chunks so they're easy to handle in Telegram.
- **Fast & Reliable**: Powered by `yt-dlp` and `ffmpeg` for high-quality audio extraction.
- **Docker Ready**: Super simple to host yourself.

## 🚀 Getting Started

### 1. Create your Telegram Bot
Before you can run the bot, you need your own token from Telegram:
1. Start a chat with [@BotFather](https://t.me/BotFather).
2. Send `/newbot` and follow the prompts to name your bot.
3. BotFather will give you a **Bot Token**. Keep this safe!

### 2. Deploy with Docker
The quickest way to get up and running is with Docker. You'll need to build the image and then run it.

First, build the project (you'll need Go installed for this step):
```bash
go build -o tube2pod .
```

Then, build the Docker image:
```bash
docker build -t tube2pod .
```

Finally, run your bot:
```bash
docker run -d \
  --name tube2pod \
  -e TELEGRAM_BOT_TOKEN="YOUR_BOT_TOKEN_HERE" \
  -v $(pwd)/tmp:/app/tmp \
  tube2pod
```

## 🛠 Usage
Once your bot is running:
1. Open your bot in Telegram and hit `/start`.
2. Paste any YouTube link.
3. The bot will download, convert, and send you the audio files.
4. Need to free up some space? Send `/cleanup` to remove temporary files from the server.

## 👩‍💻 For Developers
Want to play with the code? Here's the lowdown:

### Prerequisites
- [Go](https://go.dev/) (1.24+)
- [ffmpeg](https://ffmpeg.org/) installed on your path
- [yt-dlp](https://github.com/yt-dlp/yt-dlp) installed on your path

### Local Setup
1. Clone the repo: `git clone https://github.com/plotnikau/tube2pod.git`
2. Install dependencies: `go mod download`
3. Set your token: `export TELEGRAM_BOT_TOKEN="your_token"`
4. Run it: `go run main.go`

### Running Tests
Keep things stable by running:
```bash
go test -v ./...
```

## 📜 License
MIT License. See `LICENSE.txt` for details.

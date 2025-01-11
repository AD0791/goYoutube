# goYoutube


## Dependencies

- **Go 1.18+** (recommended)
- **[Cobra](https://github.com/spf13/cobra)** (`go get github.com/spf13/cobra`)
- **[kkdai/youtube](https://github.com/kkdai/youtube/v2)** (`go get github.com/kkdai/youtube/v2`)
- **ffmpeg** (optional, for merging separate audio/video streams):
  - **Linux (Debian/Ubuntu):** `sudo apt-get install ffmpeg`
  - **macOS (Homebrew):** `brew install ffmpeg`
  - **Windows:** Download from [ffmpeg.org](https://ffmpeg.org/download.html) or [gyan.dev/ffmpeg/builds](https://www.gyan.dev/ffmpeg/builds/)

---

## Building the Binary

```bash
go build -o GoYoutube
```

## DEPLOY  Linux/macOS

You can also install globally:

- `sudo mv GoYoutube /usr/local/bin/`

Remove it with :

- `sudo rm -rf /usr/local/bin/GoYoutube`

## USAGE

- Linux/Macos: `./GoYoutube [command] [flags]`
- Windows: `GoYoutube.exe [command] [flags]`


### single video

```bash
GoYoutube download \
  --url "https://www.youtube.com/watch?v=VIDEO_ID" \
  --out "./videos"
```

### A playlist

```bash
GoYoutube download \
  --url "https://www.youtube.com/playlist?list=PLAYLIST_ID" \
  --out "./playlist_videos" \
  --concurrency 4 \
  --skip-existing
```

Where:

`--concurrency` sets number of parallel downloads.
`--skip-existing` avoids re-downloading files that already exist.
package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	youtube "github.com/kkdai/youtube/v2"
	"github.com/spf13/cobra"

	// Import your custom client
	newclient "github.com/AD0791/GoYoutube/cmd/newclient"
)

var (
	inputURL     string
	outputDir    string
	concurrency  int
	skipExisting bool
)

// downloadCmd is our Cobra subcommand ("GoYoutube download ...")
var downloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a single YouTube video or an entire playlist",
	Long: `Download a single YouTube video or a full playlist with optional concurrency 
and the ability to skip existing files. Supports basic progressive downloads 
or separate audio/video merging (with custom logic).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// This is called when the user runs: `GoYoutube download ...`
		return runDownload()
	},
}

func init() {
	// Add the download command to the root
	rootCmd.AddCommand(downloadCmd)

	// Register flags
	downloadCmd.Flags().StringVarP(&inputURL, "url", "u", "", "YouTube URL (video or playlist) [required]")
	downloadCmd.Flags().StringVarP(&outputDir, "out", "o", "./downloads", "Output directory")
	downloadCmd.Flags().IntVarP(&concurrency, "concurrency", "c", 4, "Number of parallel downloads")
	downloadCmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip if output file already exists")

	// Mark the url as required
	downloadCmd.MarkFlagRequired("url")
}

// Example function to demonstrate usage of newclient.MyClient
func runDownload() error {
	// 1) Create your extended client
	mc := &newclient.MyClient{
		Client: &youtube.Client{}, // the original client
	}

	// 2) Check if it’s a playlist
	if isPlaylist(inputURL) {
		// Fetch minimal playlist info
		playlist, err := mc.GetPlaylist(inputURL)
		if err != nil {
			return fmt.Errorf("GetPlaylist error: %w", err)
		}

		// 3) Grab all items by paging
		entries, err := fetchAllPlaylistEntries(mc, playlist.ID)
		if err != nil {
			return fmt.Errorf("error fetching playlist entries: %w", err)
		}
		log.Printf("Found %d items in %q\n", len(entries), playlist.Title)

		// 4) Download them in parallel
		ch := make(chan youtube.PlaylistEntry, len(entries))
		for _, e := range entries {
			ch <- e
		}
		close(ch)

		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for item := range ch {
					if item.ID == "" {
						log.Printf("[worker %d] Skipping missing ID\n", workerID)
						continue
					}
					// Build watch URL => "https://www.youtube.com/watch?v=" + item.ID
					videoURL := "https://www.youtube.com/watch?v=" + item.ID
					// Build output file path
					safePlaylist := sanitizeFilename(playlist.Title)
					safeVideo := sanitizeFilename(item.Title)
					outFile := filepath.Join(outputDir, safePlaylist, safeVideo+".mp4")

					if skipExisting && fileExists(outFile) {
						log.Printf("[worker %d] Skipping %s (exists)\n", workerID, outFile)
						continue
					}

					log.Printf("[worker %d] Downloading: %s => %s\n", workerID, item.Title, outFile)
					if err := downloadSingleVideo(mc.Client, videoURL, outFile); err != nil {
						log.Printf("[worker %d] Error: %v\n", workerID, err)
					}
				}
			}(i + 1)
		}
		wg.Wait()

		log.Println("Done with playlist downloads.")
	} else {
		// Single video path
		outFile := filepath.Join(outputDir, "output.mp4")
		if skipExisting && fileExists(outFile) {
			log.Printf("Skipping %s (already exists)\n", outFile)
			return nil
		}

		if err := downloadSingleVideo(mc.Client, inputURL, outFile); err != nil {
			return fmt.Errorf("error downloading single video: %w", err)
		}
	}

	return nil
}

// fetchAllPlaylistEntries calls our new pagination method repeatedly
func fetchAllPlaylistEntries(mc *newclient.MyClient, playlistID string) ([]youtube.PlaylistEntry, error) {
	var all []youtube.PlaylistEntry
	pageToken := ""

	for {
		paged, err := mc.GetPlaylistPageToken(playlistID, pageToken)
		if err != nil {
			return nil, err
		}
		all = append(all, paged.Entries...)
		if paged.NextPageToken == "" {
			break
		}
		pageToken = paged.NextPageToken
	}

	return all, nil
}

// downloadSingleVideo handles a single video. If you previously used f.Resolution,
// swap to f.QualityLabel (which might be "1080p", etc.).
func downloadSingleVideo(c *youtube.Client, videoURL, outFile string) error {
	vid, err := c.GetVideo(videoURL)
	if err != nil {
		return fmt.Errorf("GetVideo failed: %w", err)
	}

	// Pick a progressive format with the highest QualityLabel + audioChannels > 0
	var best *youtube.Format
	for i := range vid.Formats {
		f := &vid.Formats[i]
		// e.g. f.QualityLabel => "1080p", "720p", "480p", etc.
		// check if there's audio
		if f.AudioChannels > 0 && f.QualityLabel != "" {
			if best == nil || parseRes(f.QualityLabel) > parseRes(best.QualityLabel) {
				best = f
			}
		}
	}

	if best == nil {
		return fmt.Errorf("no progressive format with audio found (need separate audio/video merge?)")
	}

	stream, total, err := c.GetStream(vid, best)
	if err != nil {
		return fmt.Errorf("GetStream failed: %w", err)
	}
	defer stream.Close()

	if err := os.MkdirAll(filepath.Dir(outFile), 0o755); err != nil {
		return err
	}
	file, err := os.Create(outFile)
	if err != nil {
		return err
	}
	defer file.Close()

	written, err := io.Copy(file, stream)
	if err != nil {
		return err
	}
	log.Printf("Downloaded %s => %d bytes (expected ~%d)\n", outFile, written, total)
	return nil
}

// parseRes turns "1080p" into 1080, ignoring any suffix
func parseRes(label string) int {
	label = strings.TrimSuffix(label, "p")
	val, _ := strconv.Atoi(label)
	return val
}

func isPlaylist(u string) bool {
	return strings.Contains(u, "list=")
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	return strings.TrimSpace(s)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

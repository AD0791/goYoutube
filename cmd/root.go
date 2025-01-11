package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "GoYoutube",
	Short: "A YouTube video downloader CLI",
	Long: `GoYoutube is a CLI tool to download YouTube videos or playlists in high 
resolution, optionally merging separate audio/video streams via ffmpeg.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatalf("Cant start the basic execution")
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Example:
	// rootCmd.PersistentFlags().BoolVar(&someVar, "someFlag", false, "A global flag for all subcommands")
}

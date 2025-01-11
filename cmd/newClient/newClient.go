package newclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	youtube "github.com/kkdai/youtube/v2"
)

// MyClient extends the official youtube.Client (embedded) so you can still call
// methods like GetPlaylist, GetVideo, GetStream, etc. from MyClient.
type MyClient struct {
	*youtube.Client
}

// PlaylistPaged is a single page of results from a playlist, plus the next-page token.
type PlaylistPaged struct {
	PlaylistID    string
	Entries       []youtube.PlaylistEntry
	NextPageToken string
}

// GetPlaylistPageToken is a PUBLIC method that fetches up to 100 items from a playlist.
// If pageToken is empty, it fetches the first page; otherwise it fetches the continuation page.
func (mc *MyClient) GetPlaylistPageToken(playlistID, pageToken string) (*PlaylistPaged, error) {
	// We'll build the request to the YouTube "browse" endpoint ourselves.
	// Because we can't call private methods (like mc.httpPostBodyBytes).
	// We'll do a minimal approach with net/http and parse JSON.

	// 1) Build a POST request to: https://www.youtube.com/youtubei/v1/browse?key=...
	// Typically, to get the first page => BrowseID = "VL" + playlistID
	// to get continuation => pass "continuation": pageToken

	var requestBody = map[string]interface{}{
		"context": map[string]interface{}{
			"client": map[string]interface{}{
				"hl":            "en",
				"gl":            "US",
				"clientName":    "WEB",
				"clientVersion": "2.20210721.00.00",
			},
		},
		"contentCheckOk": true,
		"racyCheckOk":    true,
	}

	if pageToken == "" {
		// first page
		requestBody["browseId"] = "VL" + playlistID
	} else {
		// continuation
		requestBody["continuation"] = pageToken
	}

	// 2) Marshal to JSON
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("json.Marshal failed: %w", err)
	}

	// 3) Build the request
	apiURL := "https://www.youtube.com/youtubei/v1/browse?key=" + "AIzaSyAO_FJ2SlqU8Q4STEHLGCilw_Y9_11qcW8" // the "key" used by older Web clients
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, apiURL, strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 4) Perform the request with the embedded client’s HTTPClient if set
	httpClient := mc.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("playlist page request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// 5) Parse the response JSON to find items + next page token
	bodyBytes := make([]byte, 0)
	bodyBytes, err = ioReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read resp.Body failed: %w", err)
	}

	paged, err := parsePlaylistJSON(bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("parsePlaylistJSON failed: %w", err)
	}
	paged.PlaylistID = playlistID

	return paged, nil
}

// parsePlaylistJSON is a minimal JSON parse function to extract:
// - Each video’s ID + Title
// - The continuation token
func parsePlaylistJSON(jsonData []byte) (*PlaylistPaged, error) {
	// YouTube’s JSON is deeply nested. We just find what we need.

	var root struct {
		ContinuationContents struct {
			PlaylistVideoListContinuation struct {
				Contents []struct {
					PlaylistVideoRenderer struct {
						VideoID string `json:"videoId"`
						Title   struct {
							SimpleText string `json:"simpleText"`
						} `json:"title"`
					} `json:"playlistVideoRenderer"`
				} `json:"contents"`
				Continuations []struct {
					NextContinuationData struct {
						Continuation string `json:"continuation"`
					} `json:"nextContinuationData"`
				} `json:"continuations"`
			} `json:"playlistVideoListContinuation"`
		} `json:"continuationContents"`
		Contents struct {
			TwoColumnBrowseResultsRenderer struct {
				Tabs []struct {
					TabRenderer struct {
						Content struct {
							SectionListRenderer struct {
								Contents []struct {
									ItemSectionRenderer struct {
										Contents []struct {
											PlaylistVideoListRenderer struct {
												Contents []struct {
													PlaylistVideoRenderer struct {
														VideoID string `json:"videoId"`
														Title   struct {
															SimpleText string `json:"simpleText"`
														} `json:"title"`
													} `json:"playlistVideoRenderer"`
												} `json:"contents"`
												Continuations []struct {
													NextContinuationData struct {
														Continuation string `json:"continuation"`
													} `json:"nextContinuationData"`
												} `json:"continuations"`
											} `json:"playlistVideoListRenderer"`
										} `json:"contents"`
									} `json:"itemSectionRenderer"`
								} `json:"contents"`
							} `json:"sectionListRenderer"`
						} `json:"content"`
					} `json:"tabRenderer"`
				} `json:"tabs"`
			} `json:"twoColumnBrowseResultsRenderer"`
		} `json:"contents"`
	}

	if err := json.Unmarshal(jsonData, &root); err != nil {
		return nil, err
	}

	var out PlaylistPaged
	// Attempt to parse either continuationContents or the initial "contents"
	// 1) Check continuationContents
	cc := root.ContinuationContents.PlaylistVideoListContinuation
	if len(cc.Contents) > 0 {
		// This is a continuation page
		for _, item := range cc.Contents {
			vid := item.PlaylistVideoRenderer.VideoID
			title := item.PlaylistVideoRenderer.Title.SimpleText
			if vid == "" {
				continue
			}
			out.Entries = append(out.Entries, youtube.PlaylistEntry{
				ID:    vid,
				Title: title,
			})
		}
		// next token
		for _, cont := range cc.Continuations {
			if cont.NextContinuationData.Continuation != "" {
				out.NextPageToken = cont.NextContinuationData.Continuation
				break
			}
		}
	} else {
		// Possibly the first page
		tabs := root.Contents.TwoColumnBrowseResultsRenderer.Tabs
		for _, tab := range tabs {
			sections := tab.TabRenderer.Content.SectionListRenderer.Contents
			for _, section := range sections {
				list := section.ItemSectionRenderer.Contents
				for _, c := range list {
					plRenderer := c.PlaylistVideoListRenderer
					for _, item := range plRenderer.Contents {
						vid := item.PlaylistVideoRenderer.VideoID
						title := item.PlaylistVideoRenderer.Title.SimpleText
						if vid == "" {
							continue
						}
						out.Entries = append(out.Entries, youtube.PlaylistEntry{
							ID:    vid,
							Title: title,
						})
					}
					// next token
					for _, cont := range plRenderer.Continuations {
						if cont.NextContinuationData.Continuation != "" {
							out.NextPageToken = cont.NextContinuationData.Continuation
							break
						}
					}
				}
			}
		}
	}

	return &out, nil
}

// ioReadAll is a simple replacement for io.ReadAll, in case your environment is older Go.
func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

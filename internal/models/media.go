// Package models defines the domain types shared across the application.
package models

// Artist represents a music artist.
type Artist struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image,omitempty"`
	Share string `json:"share,omitempty"`
}

// Album represents a music album.
type Album struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Image  string   `json:"image,omitempty"`
	Artist Artist   `json:"artist"`
	Year   int      `json:"year,omitempty"`
	Genres []string `json:"genres,omitempty"`
	Share  string   `json:"share,omitempty"`
}

// Track represents a single playable track.
type Track struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Duration  int      `json:"duration"` // seconds
	Artists   []Artist `json:"artists"`
	Album     Album    `json:"album"`
	Image     string   `json:"image,omitempty"`
	ISRC      string   `json:"isrc,omitempty"`
	Share     string   `json:"share,omitempty"`
	Genres    []string `json:"genres,omitempty"`
	Available bool     `json:"available"`
	HasLyrics bool     `json:"hasLyrics"`
}

// ArtistNames returns a comma separated list of artist names.
func (t Track) ArtistNames() string {
	names := make([]string, 0, len(t.Artists))
	for _, a := range t.Artists {
		names = append(names, a.Name)
	}
	return join(names, ", ")
}

func join(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}

// Playlist represents a music playlist.
type Playlist struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	TracksCount int    `json:"tracksCount,omitempty"`
	IsOwned     bool   `json:"isOwned"`
	Share       string `json:"share,omitempty"`
}

// SearchResults aggregates search results across media types.
type SearchResults struct {
	Tracks    []Track    `json:"tracks"`
	Albums    []Album    `json:"albums"`
	Artists   []Artist   `json:"artists"`
	Playlists []Playlist `json:"playlists"`
}

// Lyrics holds plain and synchronized (LRC) lyric text for a track.
type Lyrics struct {
	Plain string      `json:"plain"`
	Lines []LyricLine `json:"lines"`
}

// LyricLine is a single timed lyric line.
type LyricLine struct {
	StartMs int    `json:"startMs"`
	Text    string `json:"text"`
}

// StreamDetails holds the playback URL for a track.
type StreamDetails struct {
	URL     string `json:"url"`
	BitRate int    `json:"bitRate,omitempty"`
}

// RecommendationFolder is a named group of recommended items.
type RecommendationFolder struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Subtitle string  `json:"subtitle,omitempty"`
	Tracks   []Track `json:"tracks,omitempty"`
	Albums   []Album `json:"albums,omitempty"`
}

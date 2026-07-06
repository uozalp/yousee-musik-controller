package yousee

import (
	"encoding/json"
	"strconv"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
)

// gqlGenre normalises the YouSee "genre" field, which the API returns as
// either a single string or an array of strings.
type gqlGenre []string

func (g *gqlGenre) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '[' {
		var arr []string
		if err := json.Unmarshal(b, &arr); err != nil {
			return err
		}
		*g = arr
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	if s != "" {
		*g = []string{s}
	}
	return nil
}

// --- raw GraphQL shapes ---

type gqlArtist struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Cover string `json:"cover"`
	Share string `json:"share"`
}

type gqlArtistList struct {
	Items []gqlArtist `json:"items"`
}

type gqlAlbum struct {
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Cover       string        `json:"cover"`
	Genre       gqlGenre      `json:"genre"`
	ReleaseDate string        `json:"releaseDate"`
	Share       string        `json:"share"`
	Artist      *gqlArtist    `json:"artist"`
	Featured    gqlArtistList `json:"featuredArtists"`
	TracksCount int           `json:"tracksCount"`
}

type gqlTrack struct {
	ID                string        `json:"id"`
	Title             string        `json:"title"`
	Duration          int           `json:"duration"`
	Cover             string        `json:"cover"`
	ISRC              string        `json:"isrc"`
	Share             string        `json:"share"`
	Genre             gqlGenre      `json:"genre"`
	AvailableToStream *bool         `json:"availableToStream"`
	Artist            *gqlArtist    `json:"artist"`
	Featured          gqlArtistList `json:"featuredArtists"`
	Album             *gqlAlbum     `json:"album"`
	Lyrics            *struct {
		ID string `json:"id"`
	} `json:"lyrics"`
}

type gqlPlaylist struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Cover       string `json:"cover"`
	TracksCount int    `json:"tracksCount"`
	IsOwned     bool   `json:"isOwned"`
	Share       string `json:"share"`
}

type gqlPageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

// --- converters ---

func (a *gqlArtist) toModel() models.Artist {
	if a == nil {
		return models.Artist{}
	}
	return models.Artist{ID: a.ID, Name: a.Title, Image: a.Cover, Share: a.Share}
}

func (a *gqlAlbum) toModel() models.Album {
	if a == nil {
		return models.Album{}
	}
	album := models.Album{
		ID:     a.ID,
		Name:   a.Title,
		Image:  a.Cover,
		Genres: a.Genre,
		Share:  a.Share,
	}
	if a.Artist != nil {
		album.Artist = a.Artist.toModel()
	}
	if len(a.ReleaseDate) >= 4 {
		if y, err := strconv.Atoi(a.ReleaseDate[:4]); err == nil {
			album.Year = y
		}
	}
	return album
}

func (t *gqlTrack) toModel() models.Track {
	if t == nil {
		return models.Track{}
	}
	track := models.Track{
		ID:        t.ID,
		Title:     t.Title,
		Duration:  t.Duration,
		Image:     t.Cover,
		ISRC:      t.ISRC,
		Share:     t.Share,
		Genres:    t.Genre,
		Available: t.AvailableToStream == nil || *t.AvailableToStream,
		HasLyrics: t.Lyrics != nil && t.Lyrics.ID != "",
	}
	if t.Artist != nil {
		track.Artists = append(track.Artists, t.Artist.toModel())
	}
	for i := range t.Featured.Items {
		track.Artists = append(track.Artists, t.Featured.Items[i].toModel())
	}
	if t.Album != nil {
		track.Album = t.Album.toModel()
		if track.Image == "" {
			track.Image = t.Album.Cover
		}
	}
	return track
}

func (p *gqlPlaylist) toModel() models.Playlist {
	if p == nil {
		return models.Playlist{}
	}
	return models.Playlist{
		ID:          p.ID,
		Title:       p.Title,
		Description: p.Description,
		Image:       p.Cover,
		TracksCount: p.TracksCount,
		IsOwned:     p.IsOwned,
		Share:       p.Share,
	}
}

func tracksToModels(items []gqlTrack) []models.Track {
	out := make([]models.Track, 0, len(items))
	for i := range items {
		out = append(out, items[i].toModel())
	}
	return out
}

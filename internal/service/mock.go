package service

import (
	"context"
	"strings"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
)

// mockService is a fully offline implementation used when no YouSee
// credentials are configured. It lets the UI be exercised end to end.
type mockService struct {
	tracks    []models.Track
	albums    []models.Album
	artists   []models.Artist
	playlists []models.Playlist
}

func img(seed string) string {
	return "https://picsum.photos/seed/" + seed + "/512"
}

// NewMock returns a MusicService backed by static demo data.
func NewMock() MusicService {
	artists := []models.Artist{
		{ID: "a1", Name: "The Midnight", Image: img("midnight")},
		{ID: "a2", Name: "Bonobo", Image: img("bonobo")},
		{ID: "a3", Name: "Rufus Du Sol", Image: img("rufus")},
		{ID: "a4", Name: "Ólafur Arnalds", Image: img("olafur")},
		{ID: "a5", Name: "Tycho", Image: img("tycho")},
	}
	albumFor := func(id, name, artSeed string, ar models.Artist, year int) models.Album {
		return models.Album{ID: id, Name: name, Image: img(artSeed), Artist: ar, Year: year}
	}
	al1 := albumFor("al1", "Endless Summer", "endless", artists[0], 2016)
	al2 := albumFor("al2", "Migration", "migration", artists[1], 2017)
	al3 := albumFor("al3", "Solace", "solace", artists[2], 2018)
	al4 := albumFor("al4", "re:member", "remember", artists[3], 2018)
	al5 := albumFor("al5", "Awake", "awake", artists[4], 2014)

	tr := func(id, title string, dur int, ar models.Artist, al models.Album) models.Track {
		return models.Track{ID: id, Title: title, Duration: dur, Artists: []models.Artist{ar}, Album: al, Image: al.Image, Available: true, HasLyrics: true}
	}
	tracks := []models.Track{
		tr("t1", "Sunset", 268, artists[0], al1),
		tr("t2", "Gloria", 241, artists[0], al1),
		tr("t3", "Kerala", 223, artists[1], al2),
		tr("t4", "Bambro Koyo Ganda", 314, artists[1], al2),
		tr("t5", "No Place", 259, artists[2], al3),
		tr("t6", "Underwater", 291, artists[2], al3),
		tr("t7", "re:member", 246, artists[3], al4),
		tr("t8", "unfold", 217, artists[3], al4),
		tr("t9", "Awake", 372, artists[4], al5),
		tr("t10", "Montana", 312, artists[4], al5),
		tr("t11", "Los Angeles", 285, artists[0], al1),
		tr("t12", "Outlier", 264, artists[1], al2),
	}
	playlists := []models.Playlist{
		{ID: "p1", Title: "Late Night Drive", Description: "Synthwave & chill", Image: img("drive"), TracksCount: 42, IsOwned: true},
		{ID: "p2", Title: "Focus Flow", Description: "Deep concentration", Image: img("focus"), TracksCount: 88, IsOwned: true},
		{ID: "p3", Title: "Sunday Morning", Description: "Easy listening", Image: img("sunday"), TracksCount: 31, IsOwned: false},
		{ID: "p4", Title: "Workout Energy", Description: "High tempo hits", Image: img("workout"), TracksCount: 55, IsOwned: false},
	}
	return &mockService{tracks: tracks, albums: []models.Album{al1, al2, al3, al4, al5}, artists: artists, playlists: playlists}
}

func (m *mockService) Available() bool { return false }

func (m *mockService) Search(_ context.Context, query string, limit int) (models.SearchResults, error) {
	q := strings.ToLower(strings.TrimSpace(query))
	res := models.SearchResults{}
	for _, t := range m.tracks {
		if q == "" || strings.Contains(strings.ToLower(t.Title), q) || strings.Contains(strings.ToLower(t.ArtistNames()), q) {
			res.Tracks = append(res.Tracks, t)
		}
	}
	for _, a := range m.albums {
		if q == "" || strings.Contains(strings.ToLower(a.Name), q) || strings.Contains(strings.ToLower(a.Artist.Name), q) {
			res.Albums = append(res.Albums, a)
		}
	}
	for _, a := range m.artists {
		if q == "" || strings.Contains(strings.ToLower(a.Name), q) {
			res.Artists = append(res.Artists, a)
		}
	}
	for _, p := range m.playlists {
		if q == "" || strings.Contains(strings.ToLower(p.Title), q) {
			res.Playlists = append(res.Playlists, p)
		}
	}
	res.Tracks = capTracks(res.Tracks, limit)
	return res, nil
}

func capTracks(t []models.Track, limit int) []models.Track {
	if limit > 0 && len(t) > limit {
		return t[:limit]
	}
	return t
}

func (m *mockService) GetTrack(_ context.Context, id string) (models.Track, error) {
	for _, t := range m.tracks {
		if t.ID == id {
			return t, nil
		}
	}
	return models.Track{}, nil
}

func (m *mockService) GetAlbum(_ context.Context, id string) (models.Album, error) {
	for _, a := range m.albums {
		if a.ID == id {
			return a, nil
		}
	}
	return models.Album{}, nil
}

func (m *mockService) GetAlbumTracks(_ context.Context, id string) ([]models.Track, error) {
	var out []models.Track
	for _, t := range m.tracks {
		if t.Album.ID == id {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockService) GetArtist(_ context.Context, id string) (models.Artist, error) {
	for _, a := range m.artists {
		if a.ID == id {
			return a, nil
		}
	}
	return models.Artist{}, nil
}

func (m *mockService) GetArtistTopTracks(_ context.Context, id string) ([]models.Track, error) {
	var out []models.Track
	for _, t := range m.tracks {
		if len(t.Artists) > 0 && t.Artists[0].ID == id {
			out = append(out, t)
		}
	}
	return out, nil
}

func (m *mockService) GetPlaylist(_ context.Context, id string) (models.Playlist, error) {
	for _, p := range m.playlists {
		if p.ID == id {
			return p, nil
		}
	}
	return models.Playlist{}, nil
}

func (m *mockService) GetPlaylistTracks(_ context.Context, _ string) ([]models.Track, error) {
	return m.tracks, nil
}

func (m *mockService) GetLibraryPlaylists(_ context.Context) ([]models.Playlist, error) {
	return m.playlists, nil
}

func (m *mockService) GetLibraryTracks(_ context.Context) ([]models.Track, error) {
	return m.tracks, nil
}

func (m *mockService) GetLibraryAlbums(_ context.Context) ([]models.Album, error) {
	return m.albums, nil
}

func (m *mockService) GetLibraryArtists(_ context.Context) ([]models.Artist, error) {
	return m.artists, nil
}

func (m *mockService) GetRecommendations(_ context.Context) ([]models.RecommendationFolder, error) {
	return []models.RecommendationFolder{
		{ID: "r1", Name: "Weekly Discoveries", Subtitle: "Fresh picks for you", Tracks: m.tracks[:6]},
		{ID: "r2", Name: "Your Mix", Subtitle: "Based on your listening", Tracks: m.tracks[3:9]},
		{ID: "r3", Name: "Chill Vibes", Subtitle: "Wind down", Tracks: m.tracks[6:]},
	}, nil
}

func (m *mockService) GetLyrics(_ context.Context, _ string) (models.Lyrics, error) {
	return models.Lyrics{
		Plain: "This is a demo track.\nLyrics are not available in demo mode.\nConnect YouSee Musik to see synced lyrics.",
		Lines: []models.LyricLine{
			{StartMs: 0, Text: "This is a demo track."},
			{StartMs: 4000, Text: "Lyrics are not available in demo mode."},
			{StartMs: 8000, Text: "Connect YouSee Musik to see synced lyrics."},
		},
	}, nil
}

func (m *mockService) GetStreamDetails(_ context.Context, _ string) (models.StreamDetails, error) {
	// No real stream in demo mode; the Go player simulates progress.
	return models.StreamDetails{}, nil
}

func (m *mockService) SetFavorite(_ context.Context, _, _ string, _ bool) error {
	return nil
}

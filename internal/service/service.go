// Package service defines the music service abstraction used by the rest of
// the application and provides YouSee-backed and mock implementations.
package service

import (
	"context"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
	"github.com/danske-spil/yousee-musik-controller/internal/service/yousee"
)

// MusicService is the abstraction over a music backend (YouSee Musik).
type MusicService interface {
	Available() bool

	Search(ctx context.Context, query string, limit int) (models.SearchResults, error)

	GetTrack(ctx context.Context, id string) (models.Track, error)
	GetAlbum(ctx context.Context, id string) (models.Album, error)
	GetAlbumTracks(ctx context.Context, id string) ([]models.Track, error)
	GetArtist(ctx context.Context, id string) (models.Artist, error)
	GetArtistTopTracks(ctx context.Context, id string) ([]models.Track, error)
	GetPlaylist(ctx context.Context, id string) (models.Playlist, error)
	GetPlaylistTracks(ctx context.Context, id string) ([]models.Track, error)

	GetLibraryPlaylists(ctx context.Context) ([]models.Playlist, error)
	GetLibraryTracks(ctx context.Context) ([]models.Track, error)
	GetLibraryAlbums(ctx context.Context) ([]models.Album, error)
	GetLibraryArtists(ctx context.Context) ([]models.Artist, error)

	GetRecommendations(ctx context.Context) ([]models.RecommendationFolder, error)
	GetLyrics(ctx context.Context, trackID string) (models.Lyrics, error)
	GetStreamDetails(ctx context.Context, trackID string) (models.StreamDetails, error)

	SetFavorite(ctx context.Context, mediaType, id string, favorite bool) error
}

// youSeeService adapts *yousee.Client to the MusicService interface.
type youSeeService struct {
	*yousee.Client
}

// Available reports that the real YouSee service is configured.
func (youSeeService) Available() bool { return true }

// New builds a MusicService. When credentials are provided, it returns a
// YouSee-backed service; otherwise a mock service with demo data.
func New(username, password string) MusicService {
	if username == "" || password == "" {
		return NewMock()
	}
	return youSeeService{Client: yousee.New(username, password)}
}

package yousee

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
)

// streamQuality is the requested bitrate for playback (in kbps).
const streamQuality = 320

const trackFields = `
	id
	title
	duration
	cover(size: 512)
	isrc
	share
	genre
	availableToStream
	lyrics { id }
	artist { id title cover(size: 512) share }
	featuredArtists { items { id title cover(size: 512) share } }
	album { id title cover(size: 512) }
`

const albumFields = `
	id
	title
	cover(size: 512)
	genre
	releaseDate
	share
	tracksCount
	artist { id title cover(size: 512) share }
	featuredArtists { items { id title cover(size: 512) } }
`

const artistFields = `
	id
	title
	cover(size: 512)
	share
`

const playlistFields = `
	id
	title
	description
	tracksCount
	isOwned
	share
	cover(size: 512)
`

// Search performs a mixed search across tracks, albums, artists and playlists.
func (c *Client) Search(ctx context.Context, query string, limit int) (models.SearchResults, error) {
	q := fmt.Sprintf(`
		query search($criterion: String!, $first: Int = 10) {
			search(criterion: $criterion) {
				tracks(first: $first) { items { %s } }
				albums(first: $first) { items { %s } }
				artists(first: $first) { items { %s } }
				playlists(first: $first) { items { %s } }
			}
		}`, trackFields, albumFields, artistFields, playlistFields)

	var data struct {
		Search struct {
			Tracks    struct{ Items []gqlTrack }    `json:"tracks"`
			Albums    struct{ Items []gqlAlbum }    `json:"albums"`
			Artists   struct{ Items []gqlArtist }   `json:"artists"`
			Playlists struct{ Items []gqlPlaylist } `json:"playlists"`
		} `json:"search"`
	}
	if err := c.post(ctx, q, map[string]any{"criterion": query, "first": limit}, &data); err != nil {
		return models.SearchResults{}, err
	}

	res := models.SearchResults{}
	res.Tracks = tracksToModels(data.Search.Tracks.Items)
	for i := range data.Search.Albums.Items {
		res.Albums = append(res.Albums, data.Search.Albums.Items[i].toModel())
	}
	for i := range data.Search.Artists.Items {
		res.Artists = append(res.Artists, data.Search.Artists.Items[i].toModel())
	}
	for i := range data.Search.Playlists.Items {
		res.Playlists = append(res.Playlists, data.Search.Playlists.Items[i].toModel())
	}
	return res, nil
}

// GetTrack returns full details for a single track.
func (c *Client) GetTrack(ctx context.Context, id string) (models.Track, error) {
	q := fmt.Sprintf(`query getTrack($id: ID!) { catalog { track(id: $id) { %s } } }`, trackFields)
	var data struct {
		Catalog struct {
			Track *gqlTrack `json:"track"`
		} `json:"catalog"`
	}
	if err := c.post(ctx, q, map[string]any{"id": id}, &data); err != nil {
		return models.Track{}, err
	}
	if data.Catalog.Track == nil {
		return models.Track{}, fmt.Errorf("track %s not found", id)
	}
	return data.Catalog.Track.toModel(), nil
}

// GetAlbum returns full details for a single album.
func (c *Client) GetAlbum(ctx context.Context, id string) (models.Album, error) {
	q := fmt.Sprintf(`query getAlbum($id: ID!) { catalog { album(id: $id) { %s } } }`, albumFields)
	var data struct {
		Catalog struct {
			Album *gqlAlbum `json:"album"`
		} `json:"catalog"`
	}
	if err := c.post(ctx, q, map[string]any{"id": id}, &data); err != nil {
		return models.Album{}, err
	}
	if data.Catalog.Album == nil {
		return models.Album{}, fmt.Errorf("album %s not found", id)
	}
	return data.Catalog.Album.toModel(), nil
}

// GetAlbumTracks returns all tracks belonging to an album.
func (c *Client) GetAlbumTracks(ctx context.Context, id string) ([]models.Track, error) {
	q := fmt.Sprintf(`
		query albumTracks($id: ID!, $first: Int = 50, $after: String) {
			catalog { album(id: $id) {
				tracks(first: $first, after: $after) {
					items { %s }
					pageInfo { hasNextPage endCursor }
				}
			} }
		}`, trackFields)

	var tracks []models.Track
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Catalog struct {
				Album struct {
					Tracks struct {
						Items    []gqlTrack  `json:"items"`
						PageInfo gqlPageInfo `json:"pageInfo"`
					} `json:"tracks"`
				} `json:"album"`
			} `json:"catalog"`
		}
		vars := map[string]any{"id": id, "first": pageSize}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return nil, err
		}
		tracks = append(tracks, tracksToModels(data.Catalog.Album.Tracks.Items)...)
		if !data.Catalog.Album.Tracks.PageInfo.HasNextPage {
			break
		}
		after = data.Catalog.Album.Tracks.PageInfo.EndCursor
	}
	return tracks, nil
}

// GetArtist returns details for a single artist.
func (c *Client) GetArtist(ctx context.Context, id string) (models.Artist, error) {
	q := fmt.Sprintf(`query getArtist($id: ID!) { catalog { artist(id: $id) { %s } } }`, artistFields)
	var data struct {
		Catalog struct {
			Artist *gqlArtist `json:"artist"`
		} `json:"catalog"`
	}
	if err := c.post(ctx, q, map[string]any{"id": id}, &data); err != nil {
		return models.Artist{}, err
	}
	if data.Catalog.Artist == nil {
		return models.Artist{}, fmt.Errorf("artist %s not found", id)
	}
	return data.Catalog.Artist.toModel(), nil
}

// GetArtistTopTracks returns the most popular tracks for an artist.
func (c *Client) GetArtistTopTracks(ctx context.Context, id string) ([]models.Track, error) {
	q := fmt.Sprintf(`
		query artistTop($id: ID!, $first: Int = 25) {
			catalog { artist(id: $id) {
				tracks(first: $first, orderBy: POPULARITY) { items { %s } }
			} }
		}`, trackFields)
	var data struct {
		Catalog struct {
			Artist struct {
				Tracks struct{ Items []gqlTrack } `json:"tracks"`
			} `json:"artist"`
		} `json:"catalog"`
	}
	if err := c.post(ctx, q, map[string]any{"id": id, "first": 25}, &data); err != nil {
		return nil, err
	}
	return tracksToModels(data.Catalog.Artist.Tracks.Items), nil
}

// GetPlaylist returns details for a single playlist.
func (c *Client) GetPlaylist(ctx context.Context, id string) (models.Playlist, error) {
	q := fmt.Sprintf(`query getPlaylist($id: ID!) { playlists { playlist(id: $id) { %s } } }`, playlistFields)
	var data struct {
		Playlists struct {
			Playlist *gqlPlaylist `json:"playlist"`
		} `json:"playlists"`
	}
	if err := c.post(ctx, q, map[string]any{"id": id}, &data); err != nil {
		return models.Playlist{}, err
	}
	if data.Playlists.Playlist == nil {
		return models.Playlist{}, fmt.Errorf("playlist %s not found", id)
	}
	return data.Playlists.Playlist.toModel(), nil
}

// GetPlaylistTracks returns all tracks belonging to a playlist.
func (c *Client) GetPlaylistTracks(ctx context.Context, id string) ([]models.Track, error) {
	q := fmt.Sprintf(`
		query playlistTracks($id: ID!, $first: Int = 50, $after: String) {
			playlists { playlist(id: $id) {
				tracks(first: $first, after: $after) {
					items { %s }
					pageInfo { hasNextPage endCursor }
				}
			} }
		}`, trackFields)

	var tracks []models.Track
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Playlists struct {
				Playlist struct {
					Tracks struct {
						Items    []gqlTrack  `json:"items"`
						PageInfo gqlPageInfo `json:"pageInfo"`
					} `json:"tracks"`
				} `json:"playlist"`
			} `json:"playlists"`
		}
		vars := map[string]any{"id": id, "first": pageSize}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return nil, err
		}
		tracks = append(tracks, tracksToModels(data.Playlists.Playlist.Tracks.Items)...)
		if !data.Playlists.Playlist.Tracks.PageInfo.HasNextPage {
			break
		}
		after = data.Playlists.Playlist.Tracks.PageInfo.EndCursor
	}
	return tracks, nil
}

// GetLibraryPlaylists returns the user's saved/subscribed playlists.
func (c *Client) GetLibraryPlaylists(ctx context.Context) ([]models.Playlist, error) {
	q := fmt.Sprintf(`
		query favoritePlaylists($first: Int!, $after: String) {
			me { playlists {
				combinedPlaylists(first: $first, after: $after, orderBy: MODIFIED_DATE) {
					items { %s }
					pageInfo { hasNextPage endCursor }
				}
			} }
		}`, playlistFields)

	var playlists []models.Playlist
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Me struct {
				Playlists struct {
					Combined struct {
						Items    []gqlPlaylist `json:"items"`
						PageInfo gqlPageInfo   `json:"pageInfo"`
					} `json:"combinedPlaylists"`
				} `json:"playlists"`
			} `json:"me"`
		}
		vars := map[string]any{"first": pageSize}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return nil, err
		}
		for j := range data.Me.Playlists.Combined.Items {
			playlists = append(playlists, data.Me.Playlists.Combined.Items[j].toModel())
		}
		if !data.Me.Playlists.Combined.PageInfo.HasNextPage {
			break
		}
		after = data.Me.Playlists.Combined.PageInfo.EndCursor
	}
	return playlists, nil
}

// GetLibraryTracks returns the user's favorite tracks.
func (c *Client) GetLibraryTracks(ctx context.Context) ([]models.Track, error) {
	q := fmt.Sprintf(`
		query favoriteTracks($first: Int!, $after: String) {
			me { favorites {
				tracks(first: $first, after: $after) {
					items { %s }
					pageInfo { hasNextPage endCursor }
				}
			} }
		}`, trackFields)

	var tracks []models.Track
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Me struct {
				Favorites struct {
					Tracks struct {
						Items    []gqlTrack  `json:"items"`
						PageInfo gqlPageInfo `json:"pageInfo"`
					} `json:"tracks"`
				} `json:"favorites"`
			} `json:"me"`
		}
		vars := map[string]any{"first": pageSize}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return nil, err
		}
		tracks = append(tracks, tracksToModels(data.Me.Favorites.Tracks.Items)...)
		if !data.Me.Favorites.Tracks.PageInfo.HasNextPage {
			break
		}
		after = data.Me.Favorites.Tracks.PageInfo.EndCursor
	}
	return tracks, nil
}

// GetLibraryAlbums returns the user's favorite albums.
func (c *Client) GetLibraryAlbums(ctx context.Context) ([]models.Album, error) {
	q := fmt.Sprintf(`
		query favoriteAlbums($first: Int!, $after: String) {
			me { favorites {
				albums(first: $first, after: $after) {
					items { %s }
					pageInfo { hasNextPage endCursor }
				}
			} }
		}`, albumFields)

	var albums []models.Album
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Me struct {
				Favorites struct {
					Albums struct {
						Items    []gqlAlbum  `json:"items"`
						PageInfo gqlPageInfo `json:"pageInfo"`
					} `json:"albums"`
				} `json:"favorites"`
			} `json:"me"`
		}
		vars := map[string]any{"first": pageSize}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return nil, err
		}
		for j := range data.Me.Favorites.Albums.Items {
			albums = append(albums, data.Me.Favorites.Albums.Items[j].toModel())
		}
		if !data.Me.Favorites.Albums.PageInfo.HasNextPage {
			break
		}
		after = data.Me.Favorites.Albums.PageInfo.EndCursor
	}
	return albums, nil
}

// GetLibraryArtists returns the user's favorite artists.
func (c *Client) GetLibraryArtists(ctx context.Context) ([]models.Artist, error) {
	q := fmt.Sprintf(`
		query favoriteArtists($first: Int!, $after: String) {
			me { favorites {
				artists(first: $first, after: $after) {
					items { %s }
					pageInfo { hasNextPage endCursor }
				}
			} }
		}`, artistFields)

	var artists []models.Artist
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Me struct {
				Favorites struct {
					Artists struct {
						Items    []gqlArtist `json:"items"`
						PageInfo gqlPageInfo `json:"pageInfo"`
					} `json:"artists"`
				} `json:"favorites"`
			} `json:"me"`
		}
		vars := map[string]any{"first": pageSize}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return nil, err
		}
		for j := range data.Me.Favorites.Artists.Items {
			artists = append(artists, data.Me.Favorites.Artists.Items[j].toModel())
		}
		if !data.Me.Favorites.Artists.PageInfo.HasNextPage {
			break
		}
		after = data.Me.Favorites.Artists.PageInfo.EndCursor
	}
	return artists, nil
}

// GetLyrics attempts to retrieve lyrics for a track.
func (c *Client) GetLyrics(ctx context.Context, trackID string) (models.Lyrics, error) {
	q := `
		query lyric($id: ID!, $first: Int = 200, $after: String) {
			catalog { track(id: $id) {
				lyrics { lrc(first: $first, after: $after) {
					items { startInMs durationInMs line }
					pageInfo { hasNextPage endCursor }
				} }
			} }
		}`

	var lyrics models.Lyrics
	after := ""
	for i := 0; i < maxPages; i++ {
		var data struct {
			Catalog struct {
				Track struct {
					Lyrics *struct {
						Lrc struct {
							Items []struct {
								StartInMs int    `json:"startInMs"`
								Line      string `json:"line"`
							} `json:"items"`
							PageInfo gqlPageInfo `json:"pageInfo"`
						} `json:"lrc"`
					} `json:"lyrics"`
				} `json:"track"`
			} `json:"catalog"`
		}
		vars := map[string]any{"id": trackID, "first": 200}
		if after != "" {
			vars["after"] = after
		}
		if err := c.post(ctx, q, vars, &data); err != nil {
			return models.Lyrics{}, err
		}
		if data.Catalog.Track.Lyrics == nil {
			break
		}
		for _, item := range data.Catalog.Track.Lyrics.Lrc.Items {
			lyrics.Lines = append(lyrics.Lines, models.LyricLine{StartMs: item.StartInMs, Text: item.Line})
			lyrics.Plain += item.Line + "\n"
		}
		if !data.Catalog.Track.Lyrics.Lrc.PageInfo.HasNextPage {
			break
		}
		after = data.Catalog.Track.Lyrics.Lrc.PageInfo.EndCursor
	}
	return lyrics, nil
}

var reBitrate = regexp.MustCompile(`mp4-(\d+)kbps`)

// GetStreamDetails returns the playback URL for a track.
func (c *Client) GetStreamDetails(ctx context.Context, trackID string) (models.StreamDetails, error) {
	q := `
		query playbackFull($id: ID!, $quality: StreamQuality!) {
			playback(trackId: $id) { full(quality: $quality) }
		}`
	var data struct {
		Playback struct {
			Full string `json:"full"`
		} `json:"playback"`
	}
	vars := map[string]any{"id": trackID, "quality": fmt.Sprintf("KBPS_%d", streamQuality)}
	if err := c.post(ctx, q, vars, &data); err != nil {
		return models.StreamDetails{}, err
	}
	if data.Playback.Full == "" {
		return models.StreamDetails{}, fmt.Errorf("track %s is not available for streaming", trackID)
	}
	sd := models.StreamDetails{URL: data.Playback.Full}
	if m := reBitrate.FindStringSubmatch(data.Playback.Full); m != nil {
		sd.BitRate, _ = strconv.Atoi(m[1])
	}
	return sd, nil
}

// ReportPlayback informs YouSee that a track was played for the given duration.
func (c *Client) ReportPlayback(ctx context.Context, trackID, playbackURL string, playedSeconds int) error {
	m := `
		mutation reportPlayback($report: ReportPlaybackInput!) {
			reportPlayback(report: $report) { ok }
		}`
	report := map[string]any{
		"playbackUrl":     playbackURL,
		"playbackContext": base64.StdEncoding.EncodeToString([]byte("catalog:track;" + trackID)),
		"playedSeconds":   playedSeconds,
		"playedAt":        time.Now().UTC().Format(time.RFC3339),
	}
	return c.post(ctx, m, map[string]any{"report": report}, nil)
}

// SetFavorite adds or removes a media item from the user's favorites.
// mediaType must be one of "Track", "Album", "Artist", "Playlist".
func (c *Client) SetFavorite(ctx context.Context, mediaType, id string, favorite bool) error {
	verb := "add"
	if !favorite {
		verb = "remove"
	}
	m := fmt.Sprintf(`
		mutation setFavorite($id: ID!) {
			favorites { %s%s(id: $id) { ok } }
		}`, verb, mediaType)
	return c.post(ctx, m, map[string]any{"id": id}, nil)
}

package handlers

import (
	"io/fs"
	"net/http"

	"github.com/danske-spil/yousee-musik-controller/internal/player"
	"github.com/danske-spil/yousee-musik-controller/internal/service"
	"github.com/danske-spil/yousee-musik-controller/internal/websocket"
	"github.com/danske-spil/yousee-musik-controller/web"
)

// Server holds application dependencies and HTTP handlers.
type Server struct {
	svc      service.MusicService
	player   *player.Player
	hub      *websocket.Hub
	renderer *renderer
}

// NewServer constructs a Server.
func NewServer(svc service.MusicService, pl *player.Player, hub *websocket.Hub) (*Server, error) {
	r, err := newRenderer()
	if err != nil {
		return nil, err
	}
	return &Server{svc: svc, player: pl, hub: hub, renderer: r}, nil
}

// Routes builds the HTTP handler with all routes registered.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Static assets.
	staticFS, _ := fs.Sub(web.Static, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// WebSocket.
	mux.HandleFunc("GET /ws", s.hub.ServeHTTP)

	// Full-page routes (deep links & push-url targets).
	mux.HandleFunc("GET /", s.handleIndex)
	mux.HandleFunc("GET /search", s.pageHandler("search"))
	mux.HandleFunc("GET /browse", s.pageHandler("browse"))
	mux.HandleFunc("GET /playlists", s.pageHandler("playlists"))
	mux.HandleFunc("GET /favorites", s.pageHandler("favorites"))
	mux.HandleFunc("GET /radio", s.pageHandler("radio"))

	// HTMX partials.
	mux.HandleFunc("GET /partials/nowplaying", s.partialNowPlaying)
	mux.HandleFunc("GET /partials/queue", s.partialQueue)
	mux.HandleFunc("GET /partials/playbar", s.partialPlaybar)
	mux.HandleFunc("GET /partials/search", s.partialSearch)
	mux.HandleFunc("GET /partials/search/results", s.partialSearchResults)
	mux.HandleFunc("GET /partials/browse", s.partialBrowse)
	mux.HandleFunc("GET /partials/radio", s.partialRadio)
	mux.HandleFunc("GET /partials/playlists", s.partialPlaylists)
	mux.HandleFunc("GET /partials/playlist/{id}", s.partialPlaylistDetail)
	mux.HandleFunc("GET /partials/album/{id}", s.partialAlbumDetail)
	mux.HandleFunc("GET /partials/artist/{id}", s.partialArtistDetail)
	mux.HandleFunc("GET /partials/favorites", s.partialFavorites)
	mux.HandleFunc("GET /partials/lyrics/{id}", s.partialLyrics)

	// REST API – playback commands.
	mux.HandleFunc("POST /api/toggle", s.apiToggle)
	mux.HandleFunc("POST /api/play", s.apiPlayJSON)
	mux.HandleFunc("POST /api/pause", s.apiPause)
	mux.HandleFunc("POST /api/next", s.apiNext)
	mux.HandleFunc("POST /api/previous", s.apiPrevious)
	mux.HandleFunc("POST /api/seek", s.apiSeek)
	mux.HandleFunc("POST /api/volume", s.apiVolume)
	mux.HandleFunc("POST /api/mute", s.apiMute)
	mux.HandleFunc("POST /api/shuffle", s.apiShuffle)
	mux.HandleFunc("POST /api/repeat", s.apiRepeat)

	// REST API – play contexts.
	mux.HandleFunc("POST /api/play/track/{id}", s.apiPlayTrack)
	mux.HandleFunc("POST /api/play/{ctx}/{id}", s.apiPlayContext)

	// REST API – queue.
	mux.HandleFunc("POST /api/queue/index/{i}", s.apiQueueIndex)
	mux.HandleFunc("POST /api/queue/add/track/{id}", s.apiQueueAddTrack)
	mux.HandleFunc("POST /api/queue/clear", s.apiQueueClear)

	// REST API – favorites & state.
	mux.HandleFunc("POST /api/favorite", s.apiFavorite)
	mux.HandleFunc("GET /api/player", s.apiGetPlayer)
	mux.HandleFunc("GET /api/queue", s.apiGetQueue)

	return mux
}

package handlers

import (
	"context"
	"net/http"
)

// buildView returns a ViewData with the current player state and nav set.
func (s *Server) buildView(nav string) ViewData {
	return ViewData{Nav: nav, Player: s.player.State()}
}

// handleIndex serves the full application shell (Now Playing).
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	s.renderer.page(w, s.buildView("nowplaying"))
}

// pageHandler returns a handler that renders the full shell for a given nav.
func (s *Server) pageHandler(nav string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vd := s.buildView(nav)
		s.loadPageData(r.Context(), &vd, r.URL.Query().Get("q"))
		s.renderer.page(w, vd)
	}
}

// loadPageData populates section-specific data based on the nav.
func (s *Server) loadPageData(ctx context.Context, vd *ViewData, query string) {
	switch vd.Nav {
	case "search":
		vd.Query = query
		if query != "" {
			if res, err := s.svc.Search(ctx, query, 12); err == nil {
				vd.Search = &res
			}
		}
	case "browse", "radio":
		if recs, err := s.svc.GetRecommendations(ctx); err == nil {
			vd.Recommendations = recs
		}
	case "playlists":
		if pls, err := s.svc.GetLibraryPlaylists(ctx); err == nil {
			vd.Playlists = pls
		}
	case "favorites":
		if tracks, err := s.svc.GetLibraryTracks(ctx); err == nil {
			vd.Tracks = tracks
		}
		if albums, err := s.svc.GetLibraryAlbums(ctx); err == nil {
			vd.FavAlbums = albums
		}
		if artists, err := s.svc.GetLibraryArtists(ctx); err == nil {
			vd.FavArtists = artists
		}
	}
}

// --- partial handlers ---

func (s *Server) partialNowPlaying(w http.ResponseWriter, r *http.Request) {
	s.renderer.partial(w, "nowplaying", s.buildView("nowplaying"))
}

func (s *Server) partialQueue(w http.ResponseWriter, r *http.Request) {
	s.renderer.partial(w, "queue", s.buildView("nowplaying"))
}

func (s *Server) partialPlaybar(w http.ResponseWriter, r *http.Request) {
	s.renderer.partial(w, "playbar", s.buildView("nowplaying"))
}

func (s *Server) partialSearch(w http.ResponseWriter, r *http.Request) {
	vd := s.buildView("search")
	vd.Query = r.URL.Query().Get("q")
	if vd.Query != "" {
		if res, err := s.svc.Search(r.Context(), vd.Query, 12); err == nil {
			vd.Search = &res
		}
	}
	s.renderer.partial(w, "search", vd)
}

func (s *Server) partialSearchResults(w http.ResponseWriter, r *http.Request) {
	vd := s.buildView("search")
	vd.Query = r.URL.Query().Get("q")
	if vd.Query != "" {
		if res, err := s.svc.Search(r.Context(), vd.Query, 12); err == nil {
			vd.Search = &res
		}
	}
	s.renderer.partial(w, "search-results", vd)
}

func (s *Server) partialBrowse(w http.ResponseWriter, r *http.Request) {
	vd := s.buildView("browse")
	if recs, err := s.svc.GetRecommendations(r.Context()); err == nil {
		vd.Recommendations = recs
	}
	s.renderer.partial(w, "browse", vd)
}

func (s *Server) partialRadio(w http.ResponseWriter, r *http.Request) {
	vd := s.buildView("radio")
	if recs, err := s.svc.GetRecommendations(r.Context()); err == nil {
		vd.Recommendations = recs
	}
	s.renderer.partial(w, "radio", vd)
}

func (s *Server) partialPlaylists(w http.ResponseWriter, r *http.Request) {
	vd := s.buildView("playlists")
	if pls, err := s.svc.GetLibraryPlaylists(r.Context()); err == nil {
		vd.Playlists = pls
	}
	s.renderer.partial(w, "playlists", vd)
}

func (s *Server) partialPlaylistDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vd := s.buildView("playlists")
	pl, err := s.svc.GetPlaylist(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "playlist not found")
		return
	}
	vd.Playlist = &pl
	if tracks, err := s.svc.GetPlaylistTracks(r.Context(), id); err == nil {
		vd.Tracks = tracks
	}
	s.renderer.partial(w, "playlist-detail", vd)
}

func (s *Server) partialAlbumDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vd := s.buildView("browse")
	al, err := s.svc.GetAlbum(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "album not found")
		return
	}
	vd.Album = &al
	if tracks, err := s.svc.GetAlbumTracks(r.Context(), id); err == nil {
		vd.Tracks = tracks
	}
	s.renderer.partial(w, "album-detail", vd)
}

func (s *Server) partialArtistDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vd := s.buildView("browse")
	ar, err := s.svc.GetArtist(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "artist not found")
		return
	}
	vd.Artist = &ar
	if tracks, err := s.svc.GetArtistTopTracks(r.Context(), id); err == nil {
		vd.Tracks = tracks
	}
	s.renderer.partial(w, "artist-detail", vd)
}

func (s *Server) partialFavorites(w http.ResponseWriter, r *http.Request) {
	vd := s.buildView("favorites")
	s.loadPageData(r.Context(), &vd, "")
	s.renderer.partial(w, "favorites", vd)
}

func (s *Server) partialLyrics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	vd := s.buildView("nowplaying")
	ly, _ := s.svc.GetLyrics(r.Context(), id) // zero value on error is fine
	vd.Lyrics = &ly
	s.renderer.partial(w, "lyrics", vd)
}

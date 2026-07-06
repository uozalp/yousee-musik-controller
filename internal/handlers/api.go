package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
)

// ok responds 204 No Content (UI updates arrive via WebSocket).
func ok(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// bg returns a background context for player commands whose stream fetch
// outlives the HTTP request.
func bg() context.Context { return context.Background() }

func (s *Server) apiToggle(w http.ResponseWriter, r *http.Request) {
	s.player.Toggle()
	ok(w)
}

func (s *Server) apiPause(w http.ResponseWriter, r *http.Request) {
	s.player.SetPlaying(false)
	ok(w)
}

func (s *Server) apiNext(w http.ResponseWriter, r *http.Request) {
	s.player.Next(bg())
	ok(w)
}

func (s *Server) apiPrevious(w http.ResponseWriter, r *http.Request) {
	s.player.Previous(bg())
	ok(w)
}

func (s *Server) apiShuffle(w http.ResponseWriter, r *http.Request) {
	s.player.ToggleShuffle()
	ok(w)
}

func (s *Server) apiRepeat(w http.ResponseWriter, r *http.Request) {
	s.player.CycleRepeat()
	ok(w)
}

func (s *Server) apiMute(w http.ResponseWriter, r *http.Request) {
	s.player.ToggleMute()
	ok(w)
}

func (s *Server) apiSeek(w http.ResponseWriter, r *http.Request) {
	defer drain(r.Body)
	var body struct {
		PositionMs int `json:"positionMs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	s.player.Seek(body.PositionMs)
	ok(w)
}

func (s *Server) apiVolume(w http.ResponseWriter, r *http.Request) {
	defer drain(r.Body)
	var body struct {
		Volume int `json:"volume"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	s.player.SetVolume(body.Volume)
	ok(w)
}

// apiPlayJSON handles POST /api/play with a JSON body {"trackId": "..."}.
func (s *Server) apiPlayJSON(w http.ResponseWriter, r *http.Request) {
	defer drain(r.Body)
	var body struct {
		TrackID string `json:"trackId"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.TrackID != "" {
		s.playTrackByID(body.TrackID)
	} else {
		s.player.SetPlaying(true)
	}
	ok(w)
}

func (s *Server) apiPlayTrack(w http.ResponseWriter, r *http.Request) {
	s.playTrackByID(r.PathValue("id"))
	ok(w)
}

func (s *Server) playTrackByID(id string) {
	t, err := s.svc.GetTrack(bg(), id)
	if err != nil || t.ID == "" {
		return
	}
	s.player.PlayTracks(bg(), []models.Track{t}, 0)
}

// apiPlayContext plays a list of tracks resolved from a context (album,
// playlist, artist, favorites, radio) starting at an optional index.
func (s *Server) apiPlayContext(w http.ResponseWriter, r *http.Request) {
	ctxName := r.PathValue("ctx")
	id := r.PathValue("id")
	_ = r.ParseForm()
	index := 0
	if v := r.FormValue("index"); v != "" {
		index, _ = strconv.Atoi(v)
	}

	var tracks []models.Track
	var err error
	switch ctxName {
	case "album":
		tracks, err = s.svc.GetAlbumTracks(bg(), id)
	case "playlist":
		tracks, err = s.svc.GetPlaylistTracks(bg(), id)
	case "artist":
		tracks, err = s.svc.GetArtistTopTracks(bg(), id)
	case "favorites":
		tracks, err = s.svc.GetLibraryTracks(bg())
	case "radio":
		tracks, err = s.radioTracks(id)
	default:
		writeError(w, http.StatusBadRequest, "unknown context")
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, "could not load tracks")
		return
	}
	s.player.PlayTracks(bg(), tracks, index)
	ok(w)
}

// radioTracks resolves a recommendation folder's tracks by id.
func (s *Server) radioTracks(id string) ([]models.Track, error) {
	recs, err := s.svc.GetRecommendations(bg())
	if err != nil {
		return nil, err
	}
	for _, f := range recs {
		if f.ID == id {
			return f.Tracks, nil
		}
	}
	return nil, nil
}

func (s *Server) apiQueueIndex(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.Atoi(r.PathValue("i"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid index")
		return
	}
	s.player.PlayQueueIndex(bg(), i)
	ok(w)
}

func (s *Server) apiQueueAddTrack(w http.ResponseWriter, r *http.Request) {
	t, err := s.svc.GetTrack(bg(), r.PathValue("id"))
	if err == nil && t.ID != "" {
		s.player.AddToQueue(bg(), []models.Track{t})
	}
	ok(w)
}

func (s *Server) apiQueueClear(w http.ResponseWriter, r *http.Request) {
	s.player.ClearQueue()
	ok(w)
}

func (s *Server) apiFavorite(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	mediaType := r.FormValue("mediaType")
	id := r.FormValue("id")
	favorite := r.FormValue("favorite") != "false"
	if mediaType == "" || id == "" {
		writeError(w, http.StatusBadRequest, "missing mediaType or id")
		return
	}
	_ = s.svc.SetFavorite(r.Context(), mediaType, id, favorite)
	ok(w)
}

func (s *Server) apiGetPlayer(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.player.State())
}

func (s *Server) apiGetQueue(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.player.State().Queue)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

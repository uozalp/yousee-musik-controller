// Package handlers wires HTTP routes to the player, service and templates.
package handlers

import (
	"fmt"
	"html/template"
	"io"
	"net/http"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
	"github.com/danske-spil/yousee-musik-controller/web"
)

// renderer parses and executes the application templates.
type renderer struct {
	tmpl *template.Template
}

func newRenderer() (*renderer, error) {
	funcs := template.FuncMap{
		"add":         func(a, b int) int { return a + b },
		"fmtDuration": fmtDuration,
		"dict":        dict,
	}
	t, err := template.New("").Funcs(funcs).ParseFS(web.Templates,
		"templates/layouts/*.html",
		"templates/partials/*.html",
	)
	if err != nil {
		return nil, err
	}
	return &renderer{tmpl: t}, nil
}

// page renders the full base layout.
func (r *renderer) page(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// partial renders a single named template fragment.
func (r *renderer) partial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := r.tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// fmtDuration formats seconds as m:ss.
func fmtDuration(seconds int) string {
	if seconds < 0 {
		seconds = 0
	}
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// dict builds a map from alternating key/value pairs for template composition.
func dict(values ...any) (map[string]any, error) {
	if len(values)%2 != 0 {
		return nil, fmt.Errorf("dict: odd number of arguments")
	}
	m := make(map[string]any, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			return nil, fmt.Errorf("dict: key must be a string")
		}
		m[key] = values[i+1]
	}
	return m, nil
}

// ViewData is the data passed to templates.
type ViewData struct {
	Nav    string
	Player models.PlayerState

	Query           string
	Search          *models.SearchResults
	Playlists       []models.Playlist
	Playlist        *models.Playlist
	Album           *models.Album
	Artist          *models.Artist
	Tracks          []models.Track
	Recommendations []models.RecommendationFolder
	FavAlbums       []models.Album
	FavArtists      []models.Artist
	Lyrics          *models.Lyrics
}

// writeJSONError is a small helper for API error responses.
func writeError(w http.ResponseWriter, status int, msg string) {
	http.Error(w, msg, status)
}

// drain safely discards a request body.
func drain(body io.ReadCloser) {
	_, _ = io.Copy(io.Discard, body)
	_ = body.Close()
}

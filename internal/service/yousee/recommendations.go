package yousee

import (
	"context"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
)

// GetRecommendations returns personalised recommendation folders.
func (c *Client) GetRecommendations(ctx context.Context) ([]models.RecommendationFolder, error) {
	q := `
		query recommendations($first: Int = 20) {
			me { recommendations {
				weeklyDiscoveries: recommendation(id: "weeklyDiscoveries") { ...RecTracks }
				trackRecommendations: recommendation(id: "discovertracks") { ...RecTracks }
				historyTopTracks: recommendation(id: "toptracks") { ...RecTracks }
				historyRecentTracks: recommendation(id: "recenttracks") { ...RecTracks }
				yourmix1: recommendation(id: "yourmix") { ...RecTracks }
				yourmix2: recommendation(id: "yourmix2") { ...RecTracks }
				yourmix3: recommendation(id: "yourmix3") { ...RecTracks }
			} }
		}
		fragment RecTracks on Recommendation {
			id
			title
			subtitle
			... on TracksRecommendation {
				tracks(first: $first) {
					items {
						id title duration cover(size: 512) isrc share genre
						artist { id title cover(size: 512) share }
						featuredArtists { items { id title cover(size: 512) } }
						album { id title cover(size: 512) }
					}
				}
			}
		}`

	type recFolder struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Subtitle string `json:"subtitle"`
		Tracks   struct {
			Items []gqlTrack `json:"items"`
		} `json:"tracks"`
	}
	var data struct {
		Me struct {
			Recommendations map[string]*recFolder `json:"recommendations"`
		} `json:"me"`
	}
	if err := c.post(ctx, q, map[string]any{"first": 20}, &data); err != nil {
		return nil, err
	}

	// Preserve a stable, meaningful ordering of folders.
	order := []string{
		"weeklyDiscoveries", "trackRecommendations",
		"yourmix1", "yourmix2", "yourmix3",
		"historyTopTracks", "historyRecentTracks",
	}
	var folders []models.RecommendationFolder
	for _, key := range order {
		f := data.Me.Recommendations[key]
		if f == nil || len(f.Tracks.Items) == 0 {
			continue
		}
		folders = append(folders, models.RecommendationFolder{
			ID:       f.ID,
			Name:     f.Title,
			Subtitle: f.Subtitle,
			Tracks:   tracksToModels(f.Tracks.Items),
		})
	}
	return folders, nil
}

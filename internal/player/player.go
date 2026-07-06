// Package player holds the authoritative playback state for the controller.
// The browser contains no business logic: it renders state pushed from here.
package player

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/danske-spil/yousee-musik-controller/internal/models"
	"github.com/danske-spil/yousee-musik-controller/internal/service"
	ws "github.com/danske-spil/yousee-musik-controller/internal/websocket"
)

// Player is the single source of truth for playback state.
type Player struct {
	mu    sync.RWMutex
	state models.PlayerState

	svc service.MusicService
	hub *ws.Hub

	lastTick time.Time
}

// New creates a Player with sensible defaults.
func New(svc service.MusicService, hub *ws.Hub) *Player {
	return &Player{
		svc: svc,
		hub: hub,
		state: models.PlayerState{
			Queue:      []models.Track{},
			QueueIndex: -1,
			Volume:     65,
			Repeat:     models.RepeatOff,
			Connected:  true,
		},
		lastTick: time.Now(),
	}
}

// State returns a snapshot copy of the current state.
func (p *Player) State() models.PlayerState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.copyLocked()
}

func (p *Player) copyLocked() models.PlayerState {
	s := p.state
	s.Queue = append([]models.Track(nil), p.state.Queue...)
	return s
}

// Snapshot returns the events describing the full current state, used to
// initialise a newly connected client.
func (p *Player) Snapshot() []ws.Event {
	s := p.State()
	return []ws.Event{
		{Type: ws.EventConnectionState, Payload: map[string]any{"connected": s.Connected}},
		{Type: ws.EventQueueChanged, Payload: s},
		{Type: ws.EventTrackChanged, Payload: s},
		{Type: ws.EventPlaybackState, Payload: s},
		{Type: ws.EventVolumeChanged, Payload: s},
		{Type: ws.EventProgress, Payload: progressPayload(s)},
	}
}

func progressPayload(s models.PlayerState) map[string]any {
	dur := 0
	if s.CurrentTrack != nil {
		dur = s.CurrentTrack.Duration
	}
	return map[string]any{
		"positionMs": s.PositionMs,
		"durationMs": dur * 1000,
		"playing":    s.Playing,
	}
}

// --- Ticker: advances playback position while playing ---

// StartTicker begins the background progress loop. Call in a goroutine.
func (p *Player) StartTicker(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	p.mu.Lock()
	p.lastTick = time.Now()
	p.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick()
		}
	}
}

func (p *Player) tick() {
	p.mu.Lock()
	now := time.Now()
	elapsed := now.Sub(p.lastTick)
	p.lastTick = now

	if !p.state.Playing || p.state.CurrentTrack == nil {
		p.mu.Unlock()
		return
	}

	p.state.PositionMs += int(elapsed.Milliseconds())
	durMs := p.state.CurrentTrack.Duration * 1000

	if durMs > 0 && p.state.PositionMs >= durMs {
		// Track finished; advance.
		p.advanceLocked()
		// Auto-advance needs to load the new track's stream URL, just like the
		// explicit Next/Previous commands do.
		trackID := ""
		if p.state.CurrentTrack != nil && p.state.StreamURL == "" {
			trackID = p.state.CurrentTrack.ID
		}
		s := p.copyLocked()
		p.mu.Unlock()
		p.broadcastTrackChanged(s)
		if trackID != "" {
			go p.fetchStream(context.Background(), trackID)
		}
		return
	}

	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventProgress, Payload: progressPayload(s)})
}

// advanceLocked moves to the next track according to repeat/shuffle. Caller holds lock.
func (p *Player) advanceLocked() {
	if p.state.Repeat == models.RepeatOne {
		p.state.PositionMs = 0
		return
	}
	next := p.state.QueueIndex + 1
	if p.state.Shuffle && len(p.state.Queue) > 1 {
		next = rand.Intn(len(p.state.Queue))
	}
	if next >= len(p.state.Queue) {
		if p.state.Repeat == models.RepeatAll && len(p.state.Queue) > 0 {
			next = 0
		} else {
			// End of queue.
			p.state.Playing = false
			p.state.PositionMs = 0
			return
		}
	}
	p.setIndexLocked(next)
}

// setIndexLocked sets the current track by queue index. Caller holds lock.
func (p *Player) setIndexLocked(i int) {
	if i < 0 || i >= len(p.state.Queue) {
		p.state.CurrentTrack = nil
		p.state.QueueIndex = -1
		p.state.StreamURL = ""
		p.state.Playing = false
		return
	}
	t := p.state.Queue[i]
	p.state.QueueIndex = i
	p.state.CurrentTrack = &t
	p.state.PositionMs = 0
	p.state.Playing = true
	p.state.StreamURL = ""
}

// fetchStream loads the stream URL for the current track (called without lock).
func (p *Player) fetchStream(ctx context.Context, trackID string) {
	sd, err := p.svc.GetStreamDetails(ctx, trackID)
	if err != nil {
		return
	}
	p.mu.Lock()
	if p.state.CurrentTrack != nil && p.state.CurrentTrack.ID == trackID {
		p.state.StreamURL = sd.URL
	}
	s := p.copyLocked()
	p.mu.Unlock()
	p.broadcastTrackChanged(s)
}

func (p *Player) broadcastTrackChanged(s models.PlayerState) {
	p.hub.Broadcast(ws.Event{Type: ws.EventTrackChanged, Payload: s})
	p.hub.Broadcast(ws.Event{Type: ws.EventPlaybackState, Payload: s})
	p.hub.Broadcast(ws.Event{Type: ws.EventQueueChanged, Payload: s})
	p.hub.Broadcast(ws.Event{Type: ws.EventProgress, Payload: progressPayload(s)})
}

// --- Commands ---

// PlayTracks replaces the queue with the given tracks and starts at startIndex.
func (p *Player) PlayTracks(ctx context.Context, tracks []models.Track, startIndex int) {
	if len(tracks) == 0 {
		return
	}
	if startIndex < 0 || startIndex >= len(tracks) {
		startIndex = 0
	}
	p.mu.Lock()
	p.state.Queue = append([]models.Track(nil), tracks...)
	p.setIndexLocked(startIndex)
	p.lastTick = time.Now()
	trackID := ""
	if p.state.CurrentTrack != nil {
		trackID = p.state.CurrentTrack.ID
	}
	s := p.copyLocked()
	p.mu.Unlock()

	p.broadcastTrackChanged(s)
	if trackID != "" {
		go p.fetchStream(ctx, trackID)
	}
}

// AddToQueue appends tracks to the end of the queue.
func (p *Player) AddToQueue(ctx context.Context, tracks []models.Track) {
	if len(tracks) == 0 {
		return
	}
	p.mu.Lock()
	wasEmpty := len(p.state.Queue) == 0
	p.state.Queue = append(p.state.Queue, tracks...)
	trackID := ""
	if wasEmpty {
		p.setIndexLocked(0)
		p.lastTick = time.Now()
		if p.state.CurrentTrack != nil {
			trackID = p.state.CurrentTrack.ID
		}
	}
	s := p.copyLocked()
	p.mu.Unlock()

	if wasEmpty {
		p.broadcastTrackChanged(s)
		if trackID != "" {
			go p.fetchStream(ctx, trackID)
		}
	} else {
		p.hub.Broadcast(ws.Event{Type: ws.EventQueueChanged, Payload: s})
	}
}

// PlayQueueIndex starts playback of the track at the given queue index.
func (p *Player) PlayQueueIndex(ctx context.Context, i int) {
	p.mu.Lock()
	p.setIndexLocked(i)
	p.lastTick = time.Now()
	trackID := ""
	if p.state.CurrentTrack != nil {
		trackID = p.state.CurrentTrack.ID
	}
	s := p.copyLocked()
	p.mu.Unlock()

	p.broadcastTrackChanged(s)
	if trackID != "" {
		go p.fetchStream(ctx, trackID)
	}
}

// Toggle flips between playing and paused.
func (p *Player) Toggle() {
	p.mu.Lock()
	if p.state.CurrentTrack != nil {
		p.state.Playing = !p.state.Playing
		p.lastTick = time.Now()
	}
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventPlaybackState, Payload: s})
}

// SetPlaying explicitly sets the play/pause state.
func (p *Player) SetPlaying(playing bool) {
	p.mu.Lock()
	if p.state.CurrentTrack != nil {
		p.state.Playing = playing
		p.lastTick = time.Now()
	}
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventPlaybackState, Payload: s})
}

// Next skips to the next track.
func (p *Player) Next(ctx context.Context) {
	p.mu.Lock()
	if len(p.state.Queue) == 0 {
		p.mu.Unlock()
		return
	}
	next := p.state.QueueIndex + 1
	if p.state.Shuffle && len(p.state.Queue) > 1 {
		next = rand.Intn(len(p.state.Queue))
	}
	if next >= len(p.state.Queue) {
		next = 0
	}
	p.setIndexLocked(next)
	p.lastTick = time.Now()
	trackID := ""
	if p.state.CurrentTrack != nil {
		trackID = p.state.CurrentTrack.ID
	}
	s := p.copyLocked()
	p.mu.Unlock()

	p.broadcastTrackChanged(s)
	if trackID != "" {
		go p.fetchStream(ctx, trackID)
	}
}

// Previous restarts the track or jumps to the previous one.
func (p *Player) Previous(ctx context.Context) {
	p.mu.Lock()
	if len(p.state.Queue) == 0 {
		p.mu.Unlock()
		return
	}
	// If more than 3s in, restart current track.
	if p.state.PositionMs > 3000 {
		p.state.PositionMs = 0
		p.lastTick = time.Now()
		s := p.copyLocked()
		p.mu.Unlock()
		p.hub.Broadcast(ws.Event{Type: ws.EventProgress, Payload: progressPayload(s)})
		return
	}
	prev := p.state.QueueIndex - 1
	if prev < 0 {
		prev = 0
	}
	p.setIndexLocked(prev)
	p.lastTick = time.Now()
	trackID := ""
	if p.state.CurrentTrack != nil {
		trackID = p.state.CurrentTrack.ID
	}
	s := p.copyLocked()
	p.mu.Unlock()

	p.broadcastTrackChanged(s)
	if trackID != "" {
		go p.fetchStream(ctx, trackID)
	}
}

// Seek jumps to a position (milliseconds) within the current track.
func (p *Player) Seek(positionMs int) {
	p.mu.Lock()
	if p.state.CurrentTrack != nil {
		durMs := p.state.CurrentTrack.Duration * 1000
		if positionMs < 0 {
			positionMs = 0
		}
		if durMs > 0 && positionMs > durMs {
			positionMs = durMs
		}
		p.state.PositionMs = positionMs
		p.lastTick = time.Now()
	}
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventProgress, Payload: progressPayload(s)})
}

// SetVolume sets the volume (0-100) and unmutes.
func (p *Player) SetVolume(v int) {
	if v < 0 {
		v = 0
	}
	if v > 100 {
		v = 100
	}
	p.mu.Lock()
	p.state.Volume = v
	p.state.Muted = v == 0
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventVolumeChanged, Payload: s})
}

// ToggleMute toggles the mute state.
func (p *Player) ToggleMute() {
	p.mu.Lock()
	p.state.Muted = !p.state.Muted
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventVolumeChanged, Payload: s})
}

// ToggleShuffle toggles shuffle mode.
func (p *Player) ToggleShuffle() {
	p.mu.Lock()
	p.state.Shuffle = !p.state.Shuffle
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventPlaybackState, Payload: s})
}

// CycleRepeat cycles repeat mode: off -> all -> one -> off.
func (p *Player) CycleRepeat() {
	p.mu.Lock()
	switch p.state.Repeat {
	case models.RepeatOff:
		p.state.Repeat = models.RepeatAll
	case models.RepeatAll:
		p.state.Repeat = models.RepeatOne
	default:
		p.state.Repeat = models.RepeatOff
	}
	s := p.copyLocked()
	p.mu.Unlock()
	p.hub.Broadcast(ws.Event{Type: ws.EventPlaybackState, Payload: s})
}

// ClearQueue removes all queued tracks and stops playback.
func (p *Player) ClearQueue() {
	p.mu.Lock()
	p.state.Queue = []models.Track{}
	p.state.QueueIndex = -1
	p.state.CurrentTrack = nil
	p.state.StreamURL = ""
	p.state.Playing = false
	p.state.PositionMs = 0
	s := p.copyLocked()
	p.mu.Unlock()
	p.broadcastTrackChanged(s)
}

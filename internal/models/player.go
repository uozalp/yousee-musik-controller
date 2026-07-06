package models

// RepeatMode describes how the queue repeats.
type RepeatMode string

const (
	// RepeatOff disables repeating.
	RepeatOff RepeatMode = "off"
	// RepeatAll repeats the whole queue.
	RepeatAll RepeatMode = "all"
	// RepeatOne repeats the current track.
	RepeatOne RepeatMode = "one"
)

// PlayerState is the authoritative playback state held by the Go server.
type PlayerState struct {
	CurrentTrack *Track     `json:"currentTrack"`
	StreamURL    string     `json:"streamUrl,omitempty"`
	Queue        []Track    `json:"queue"`
	QueueIndex   int        `json:"queueIndex"`
	PositionMs   int        `json:"positionMs"`
	Playing      bool       `json:"playing"`
	Volume       int        `json:"volume"` // 0-100
	Muted        bool       `json:"muted"`
	Shuffle      bool       `json:"shuffle"`
	Repeat       RepeatMode `json:"repeat"`
	Connected    bool       `json:"connected"`
}

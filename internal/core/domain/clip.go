package domain

// Clip is one entry in the deck's clip list.
type Clip struct {
	ID       int
	Name     string
	Timecode string
	Duration string
}

// ClipList is the ordered set of clips the controller can navigate.
type ClipList []Clip

// Len returns the number of clips.
func (c ClipList) Len() int { return len(c) }

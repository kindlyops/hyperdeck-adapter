package app

import "errors"

// ErrNotLocked is returned when a transport command arrives with no locked player.
var ErrNotLocked = errors.New("no player locked")

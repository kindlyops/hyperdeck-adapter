package domain

// Window identifies a target OS window the injector can act on.
type Window struct {
	Handle  uintptr
	Title   string
	Process string
}

// LockState is the current player-lock status.
type LockState struct {
	Locked  bool
	Profile *Profile
	Window  Window
}

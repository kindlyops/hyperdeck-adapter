package domain

// Window identifies a target OS window the injector can act on.
type Window struct {
	Handle  uintptr
	Title   string
	Process string
}

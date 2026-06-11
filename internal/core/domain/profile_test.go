package domain

import "testing"

func vlcProfile() Profile {
	return Profile{
		ID:        "vlc",
		Match:     Match{Process: []string{"vlc.exe", "VLC"}, TitleRegex: "VLC media player"},
		Injection: InjectionBackground,
		Keymap: Keymap{
			KeyPlay: {Key: "space"},
			KeyStop: {Key: "s"},
			KeyNext: {Key: "n"},
			KeyPrev: {Key: "p"},
		},
	}
}

func TestProfileMatchesWindow(t *testing.T) {
	p := vlcProfile()
	if !p.MatchesWindow(Window{Process: "vlc.exe", Title: "Big Buck Bunny - VLC media player"}) {
		t.Error("expected vlc.exe + title to match")
	}
	if p.MatchesWindow(Window{Process: "chrome.exe", Title: "VLC media player"}) {
		t.Error("wrong process should not match")
	}
	if p.MatchesWindow(Window{Process: "vlc.exe", Title: "Something Else"}) {
		t.Error("title regex mismatch should not match")
	}
}

func TestProfileMatchEmptyTitleRegexMatchesAnyTitle(t *testing.T) {
	p := Profile{Match: Match{Process: []string{"Mitti"}}}
	if !p.MatchesWindow(Window{Process: "Mitti", Title: "anything"}) {
		t.Error("empty title regex should match any title")
	}
}

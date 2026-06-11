// Command injcheck is a macOS/Windows diagnostic tool for the injector adapter:
// list on-screen windows, focus an app by pid, and send key chords. It exists to
// verify the real OS injector against a running player during on-device testing.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kindlyops/hyperdeck-adapter/internal/adapter/driven/injector"
	"github.com/kindlyops/hyperdeck-adapter/internal/core/domain"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	inj, err := injector.New()
	if err != nil {
		fail("init injector: %v", err)
	}

	switch os.Args[1] {
	case "trust":
		if injector.RequestAccessibility() {
			fmt.Println("Accessibility: granted")
		} else {
			fmt.Println("Accessibility: NOT granted — enable this binary in")
			fmt.Println("System Settings > Privacy & Security > Accessibility, then re-run.")
			os.Exit(1)
		}
	case "list":
		list(inj, strings.Join(os.Args[2:], " "))
	case "focus":
		if len(os.Args) < 3 {
			usage()
		}
		focus(inj, mustPID(os.Args[2]))
	case "keys":
		if len(os.Args) < 4 {
			usage()
		}
		keys(inj, mustPID(os.Args[2]), os.Args[3:], true)
	case "bgkeys":
		if len(os.Args) < 4 {
			usage()
		}
		keys(inj, mustPID(os.Args[2]), os.Args[3:], false)
	default:
		usage()
	}
}

func list(inj injector.Injector, filter string) {
	windows, err := inj.OpenWindows()
	if err != nil {
		fail("OpenWindows: %v", err)
	}
	filter = strings.ToLower(filter)
	fmt.Printf("%-8s  %-28s  %s\n", "PID", "PROCESS", "TITLE")
	for _, w := range windows {
		if filter != "" && !strings.Contains(strings.ToLower(w.Process), filter) &&
			!strings.Contains(strings.ToLower(w.Title), filter) {
			continue
		}
		fmt.Printf("%-8d  %-28s  %s\n", w.Handle, w.Process, w.Title)
	}
}

func focus(inj injector.Injector, pid int) {
	if err := inj.Focus(domain.Window{Handle: uintptr(pid)}); err != nil {
		fail("Focus: %v", err)
	}
	fmt.Printf("focused pid %d\n", pid)
}

func keys(inj injector.Injector, pid int, specs []string, focus bool) {
	chords := make([]domain.Chord, 0, len(specs))
	for _, s := range specs {
		c, err := domain.ParseChord(s)
		if err != nil {
			fail("parse chord %q: %v", s, err)
		}
		chords = append(chords, c)
	}
	w := domain.Window{Handle: uintptr(pid)}
	if focus {
		if err := inj.Focus(w); err != nil {
			fail("Focus: %v", err)
		}
	}
	if err := inj.SendKeys(w, chords); err != nil {
		fail("SendKeys: %v", err)
	}
	mode := "background"
	if focus {
		mode = "foreground"
	}
	fmt.Printf("sent %d chord(s) to pid %d (%s): %v\n", len(chords), pid, mode, specs)
}

func mustPID(s string) int {
	pid, err := strconv.Atoi(s)
	if err != nil {
		fail("invalid pid %q", s)
	}
	return pid
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: injcheck trust | list [filter] | focus <pid> | keys <pid> <chord...> | bgkeys <pid> <chord...>")
	os.Exit(2)
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "injcheck: "+format+"\n", a...)
	os.Exit(1)
}

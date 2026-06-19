package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// ─── ANSI ────────────────────────────────────────────────────────────────────

const (
	colorCyan   = "\033[0;36m"
	colorGreen  = "\033[0;32m"
	colorRed    = "\033[0;31m"
	colorYellow = "\033[1;33m"
	colorPurple = "\033[0;35m"
	colorDim    = "\033[2m"
	colorBold   = "\033[1m"
	colorReset  = "\033[0m"
)

// Red gradient palette (dark → bright)
var reds = []string{"52", "88", "124", "160", "196", "203"}

// ─── Print helpers ────────────────────────────────────────────────────────────

func Info(msg string) {
	fmt.Printf("  %s[*]%s %s\n", colorCyan, colorReset, msg)
}

func Ok(msg string) {
	fmt.Printf("  %s[+]%s %s\n", colorGreen, colorReset, msg)
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "  %s[!]%s %s\n", colorRed, colorReset, msg)
}

func Warn(msg string) {
	fmt.Printf("  %s[~]%s %s\n", colorYellow, colorReset, msg)
}

func Dim(msg string) {
	fmt.Printf("  %s%s%s\n", colorDim, msg, colorReset)
}

func Bold(msg string) {
	fmt.Printf("  %s%s%s\n", colorBold, msg, colorReset)
}

func Blank() {
	fmt.Println()
}

func Divider() {
	fmt.Printf("  %s%s%s\n", colorDim, strings.Repeat("─", 44), colorReset)
}

// KV prints a key → value pair, aligned.
func KV(key, value, valueColor string) {
	fmt.Printf("  %s%-10s%s %s·%s  %s%s%s\n",
		colorDim, key, colorReset,
		colorDim, colorReset,
		valueColor, value, colorReset,
	)
}

// ─── ASCII art helper ─────────────────────────────────────────────────────────

func printAsciiArt() {
	artLines := []string{
		"     ┌──┐┌──┐┌───────┐┌──┐ ┌──┐",
		"     │  └┘  ││   ┬   ││  └─┤  │",
		"     │    ┌─┘│   │   ││  ┌─┘  │",
		"     │  ┌┐  ││   ┴   ││  │└┐  │",
		"     └──┘└──┘└───────┘└──┘ └──┘",
	}
	maxWidth := 0
	for _, line := range artLines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	for i, line := range artLines {
		color := reds[i%len(reds)]
		fmt.Printf("  \033[38;5;%sm%-*s\033[0m\n", color, maxWidth, line)
	}
}

// ─── Banner (status / stop / logs / delete / version) ────────────────────────
// Sin "Isolated" — solo art + meta

func Banner() {
	fmt.Println()
	printAsciiArt()
	fmt.Println()
	fmt.Printf("  \033[38;5;135m[Meta]\033[0m Created by z1rov\n")
	fmt.Printf("  \033[38;5;135m[Meta]\033[0m \033[2mhttps://zirov.xyz\033[0m\n")
	fmt.Println()
}

// ─── StartScreen (kon start — typewriter + info block) ───────────────────────

func StartScreen(anvil string) {
	fmt.Println()
	printAsciiArt()

	// "Isolated" — letra a letra, amarillo, solo en start
	fmt.Printf("        ")
	word := "Isolated"
	for _, ch := range word {
		fmt.Printf("\033[38;5;226m%c\033[0m", ch)
		time.Sleep(80 * time.Millisecond)
	}
	fmt.Printf("\n\n")

	fmt.Printf("  \033[38;5;135m[Meta]\033[0m Created by z1rov\n")
	fmt.Printf("  \033[38;5;135m[Meta]\033[0m \033[2mhttps://zirov.xyz\033[0m\n")
	fmt.Println()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m Initializing container services:\n")
	fmt.Printf("  \033[38;5;160m[%-13s]\033[0m\033[2m::Network    host\033[0m\n", "host")
	fmt.Printf("  \033[38;5;124m[%-13s]\033[0m\033[2m::Mount      /anvil\033[0m\n", anvil)
	fmt.Println()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m \033[1mWelcome! Good luck with your pentesting ;)\033[0m\n")
	fmt.Printf("  %s%s%s\n\n", colorDim, strings.Repeat("─", 48), colorReset)
}

// ─── GoodbyeScreen (shown when container exits) ──────────────────────────────

func GoodbyeScreen() {
	fmt.Println()
	fmt.Printf("  \033[38;5;196m[kon]\033[0m \033[1mSession ended.\033[0m\n")
	fmt.Printf("  %s[*]%s Hope the hunt was good. Stay safe out there.\n", colorDim, colorReset)
	fmt.Println()
}

// ─── Spinner ──────────────────────────────────────────────────────────────────

type spinner struct {
	label string
	stop  chan struct{}
	wg    sync.WaitGroup
}

// NewSpinner starts an animated spinner. Call .Stop() when done.
func NewSpinner(label string) *spinner {
	s := &spinner{
		label: label,
		stop:  make(chan struct{}),
	}
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *spinner) run() {
	defer s.wg.Done()
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	i := 0
	for {
		select {
		case <-s.stop:
			fmt.Printf("\r\033[K")
			return
		default:
			fmt.Printf("\r  \033[38;5;196m%s\033[0m  %s%s%s",
				frames[i%len(frames)],
				colorDim, s.label, colorReset)
			i++
			time.Sleep(80 * time.Millisecond)
		}
	}
}

func (s *spinner) Stop() {
	close(s.stop)
	s.wg.Wait()
}

// ─── Layer progress (docker pull) ────────────────────────────────────────────

func LayerLine(id, status string) {
	color := colorDim
	icon := "·"

	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "pull complete"), strings.Contains(lower, "already exists"):
		color = colorGreen
		icon = "✓"
	case strings.Contains(lower, "pulling"):
		color = colorCyan
		icon = "↓"
	case strings.Contains(lower, "extract"):
		color = colorYellow
		icon = "⧗"
	case strings.Contains(lower, "verif"):
		color = colorPurple
		icon = "◈"
	}

	short := id
	if len(id) > 12 {
		short = id[:12]
	}
	fmt.Printf("  %s%s  %-14s  %s%s\n", color, icon, short, status, colorReset)
}

// ─── Usage ────────────────────────────────────────────────────────────────────

func Usage() {
	fmt.Println()
	printAsciiArt()
	fmt.Printf("        \033[38;5;226mIsolated\033[0m\n\n")

	fmt.Printf("  \033[38;5;135m[Meta]\033[0m Created by z1rov\n")
	fmt.Printf("  \033[38;5;135m[Meta]\033[0m \033[2mhttps://zirov.xyz\033[0m\n")
	fmt.Println()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m Usage: kon <command>\n")
	fmt.Println()

	type entry struct{ cmd, desc string }
	groups := []struct {
		title   string
		color   string
		entries []entry
	}{
		{
			"container", "160",
			[]entry{
				{"start", "start kon container"},
				{"stop", "stop kon container"},
				{"status", "show container status"},
				{"logs", "show container logs"},
				{"logs -f", "follow container logs"},
				{"exec <cmd>", "run command in container"},
			},
		},
		{
			"image", "135",
			[]entry{
				{"install", "pull kon image"},
				{"update", "update kon image"},
				{"delete", "remove all images & containers"},
				{"version", "show version info"},
			},
		},
		{
			"general", "220",
			[]entry{
				{"help", "show this help"},
			},
		},
	}

	for _, g := range groups {
		for _, e := range g.entries {
			fmt.Printf("  \033[38;5;%sm[%-13s]\033[0m\033[2m::\033[0m %s%-14s%s %s%s%s\n",
				g.color, g.title,
				colorBold, e.cmd, colorReset,
				colorDim, e.desc, colorReset)
		}
	}

	fmt.Println()
	fmt.Printf("  %s%s%s\n\n", colorDim, strings.Repeat("─", 48), colorReset)
}

// ─── VersionScreen ────────────────────────────────────────────────────────────

func VersionScreen(local string, localOk bool, remote string, remoteOk bool) {
	Banner()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m Checking version:\n")
	fmt.Println()

	localVal, localColor := local, "82"
	if !localOk {
		localVal, localColor = "not installed", "196"
	}
	fmt.Printf("  \033[38;5;160m[%-13s]\033[0m\033[2m::\033[0m \033[38;5;%sm%s\033[0m\n",
		"local", localColor, localVal)

	remoteVal, remoteColor := remote, "220"
	if !remoteOk {
		remoteVal, remoteColor = "unavailable", "196"
	}
	fmt.Printf("  \033[38;5;135m[%-13s]\033[0m\033[2m::\033[0m \033[38;5;%sm%s\033[0m\n",
		"remote", remoteColor, remoteVal)

	fmt.Println()

	if localOk && remoteOk {
		if local == remote {
			fmt.Printf("  \033[38;5;196m[Info]\033[0m \033[1mup to date\033[0m\n")
		} else {
			fmt.Printf("  \033[38;5;220m[Warn]\033[0m \033[1mupdate available: %s → %s — run: kon update\033[0m\n", local, remote)
		}
	}

	fmt.Printf("  %s%s%s\n\n", colorDim, strings.Repeat("─", 48), colorReset)
}

// ─── Start / Stop messages ────────────────────────────────────────────────────

func StartHeader() {
	fmt.Printf("  \033[38;5;196m[+]\033[0m %sstarting kon container%s\n", colorBold, colorReset)
}

func StartDetail(label, value string) {
	fmt.Printf("  \033[38;5;160m[•]\033[0m %s%-10s%s %s%s\n",
		colorDim, label+":", colorReset,
		colorCyan, value)
}

func StartDone() {
	fmt.Printf("  %s[✓]%s %scontainer ready%s\n", colorGreen, colorReset, colorBold, colorReset)
}

func StopHeader() {
	fmt.Printf("  \033[38;5;220m[~]\033[0m %sstopping kon container%s\n", colorBold, colorReset)
}

func StopDone() {
	fmt.Printf("  %s[+]%s %scontainer stopped%s\n", colorGreen, colorReset, colorBold, colorReset)
}

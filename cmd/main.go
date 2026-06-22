package main

import (
	"os"

	"github.com/z1rov/kon/internal/docker"
	"github.com/z1rov/kon/internal/storage"
	"github.com/z1rov/kon/internal/ui"
	"github.com/z1rov/kon/internal/updater"
)

func main() {
	if len(os.Args) < 2 {
		ui.Usage()
		os.Exit(0)
	}

	switch os.Args[1] {

	// ─── Container ─────────────────────────────────────────────────────────────
	case "start":
		docker.Start()

	case "stop":
		docker.Stop()

	case "status":
		docker.Status()

	case "logs":
		follow := len(os.Args) > 2 && os.Args[2] == "-f"
		docker.Logs(follow)

	case "exec":
		if len(os.Args) < 3 {
			ui.Error("usage: kon exec <command> [args...]")
			os.Exit(1)
		}
		docker.Exec(os.Args[2:])

	// ─── Image ─────────────────────────────────────────────────────────────────
	case "install":
		updater.Install()

	case "update":
		updater.Update()

	case "delete":
		docker.FullCleanup()

	case "version":
		updater.Version()

	// ─── System ────────────────────────────────────────────────────────────────
	case "relocate":
		if err := storage.Relocate(); err != nil {
			ui.Error(err.Error())
			os.Exit(1)
		}

	// ─── General ───────────────────────────────────────────────────────────────
	case "help", "--help", "-h":
		ui.Usage()

	default:
		ui.Error("unknown command: " + os.Args[1])
		ui.Blank()
		ui.Usage()
		os.Exit(1)
	}
}

package updater

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/z1rov/kon/internal/config"
	"github.com/z1rov/kon/internal/docker"
	"github.com/z1rov/kon/internal/ui"
)

// ─── Install ──────────────────────────────────────────────────────────────────

func Install() {
	spin := ui.NewSpinner("fetching version")
	remote, _ := RemoteVersion()
	spin.Stop()

	ui.Banner()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m installing %s\n", config.ImageName)
	fmt.Println()

	if err := docker.Pull(); err != nil {
		ui.Error("pull failed: " + err.Error())
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  \033[38;5;135m[%-13s]\033[0m\033[2m::\033[0m \033[38;5;82m%s\033[0m\n", "version", remote)
	fmt.Println()
	ui.Ok("kon installed — run: kon start")
	fmt.Printf("  \033[2m%s\033[0m\n\n", strings.Repeat("─", 48))
}

// ─── Update ───────────────────────────────────────────────────────────────────

func Update() {
	spin := ui.NewSpinner("checking versions")
	remote, errR := RemoteVersion()
	local, errL := LocalVersion()
	spin.Stop()

	ui.Banner()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m checking for updates\n")
	fmt.Println()

	if errR != nil {
		ui.Error("could not reach remote: " + errR.Error())
		os.Exit(1)
	}
	if errL != nil {
		fmt.Printf("  \033[38;5;160m[%-13s]\033[0m\033[2m::\033[0m \033[0;31m%s\033[0m\n", "local", "not installed")
		fmt.Println()
		ui.Warn("no local image found — run: kon install")
		fmt.Printf("  \033[2m%s\033[0m\n\n", strings.Repeat("─", 48))
		os.Exit(1)
	}

	fmt.Printf("  \033[38;5;160m[%-13s]\033[0m\033[2m::\033[0m \033[0;36m%s\033[0m\n", "local", local)
	fmt.Printf("  \033[38;5;135m[%-13s]\033[0m\033[2m::\033[0m \033[38;5;220m%s\033[0m\n", "remote", remote)
	fmt.Println()

	if local == remote {
		fmt.Printf("  \033[38;5;196m[Info]\033[0m \033[1malready up to date\033[0m\n")
		fmt.Printf("  \033[2m%s\033[0m\n\n", strings.Repeat("─", 48))
		return
	}

	fmt.Printf("  \033[38;5;220m[Warn]\033[0m update available: \033[0;36m%s\033[0m → \033[38;5;82m%s\033[0m\n", local, remote)
	fmt.Println()

	if err := docker.Pull(); err != nil {
		ui.Error("update failed: " + err.Error())
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("  \033[38;5;196m[Info]\033[0m cleaning up old images...\n")
	docker.PruneImages()

	fmt.Println()
	fmt.Printf("  \033[0;32m[+]\033[0m \033[1mupdated\033[0m \033[0;36m%s\033[0m → \033[38;5;82m%s\033[0m\n", local, remote)
	fmt.Printf("  \033[2m%s\033[0m\n\n", strings.Repeat("─", 48))
}

// ─── Version ──────────────────────────────────────────────────────────────────

func Version() {
	spin := ui.NewSpinner("fetching versions")
	remote, errR := RemoteVersion()
	local, errL := LocalVersion()
	spin.Stop()

	ui.VersionScreen(local, errL == nil, remote, errR == nil)
}

// ─── Version helpers ──────────────────────────────────────────────────────────

func RemoteVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(config.VersionURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(body))
	if version == "" {
		return "", fmt.Errorf("empty version file")
	}
	return version, nil
}

func LocalVersion() (string, error) {
	out, err := exec.Command(
		"docker", "run", "--rm", "--entrypoint", "cat",
		config.ImageName, "/kon/version/version.txt",
	).Output()
	if err != nil {
		return "", fmt.Errorf("image not found locally")
	}

	version := strings.TrimSpace(string(out))
	if version == "" {
		return "", fmt.Errorf("empty local version file")
	}
	return version, nil
}

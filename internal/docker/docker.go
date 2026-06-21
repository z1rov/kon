package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/z1rov/kon/internal/config"
	"github.com/z1rov/kon/internal/ui"
)

// ─── Container lifecycle ─────────────────────────────────────────────

func Start() {
	ui.StartHeader()

	if !imageExists() {
		ui.Error("image not found locally — run: kon install")
		os.Exit(1)
	}

	if isRunning() {
		ui.Warn("container already running — attaching")
		attach()
		return
	}

	// A stopped/leftover container with this name blocks `docker run`
	// from reusing the name — clear it out first.
	if exists() {
		_ = exec.Command("docker", "rm", "-f", config.ContainerName).Run()
	}

	if err := os.MkdirAll(config.AnvilDir(), 0755); err != nil {
		ui.Warn("could not create anvil dir: " + err.Error())
	}

	home, _ := os.UserHomeDir()
	xauth := home + "/.Xauthority"

	ui.StartDetail("image", config.ImageName)
	ui.StartDetail("anvil", config.AnvilDir())

	args := []string{
		"run", "-dit",
		"--name", config.ContainerName,
		"--network", "host",
		"--hostname", "kon",
		"--security-opt", "seccomp=unconfined",
		// X11 forwarding — needed so GUI tools like xfreerdp can open a
		// window on the host's display from inside the container.
		"-e", "DISPLAY=" + os.Getenv("DISPLAY"),
		"-e", "XAUTHORITY=/root/.Xauthority",
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix:rw",
		"-v", xauth + ":/root/.Xauthority:rw",
		"-v", "/etc/hosts:/etc/hosts",
		"-v", config.AnvilDir() + ":/anvil",
		config.ImageName,
		// No CMD override — the image's own entrypoint (init.sh) does its
		// setup and ends in `exec zsh --login -i`. With -dit it gets a
		// real allocated TTY (even though detached), so that zsh becomes
		// PID 1 and stays alive instead of exiting immediately.
	}

	// Best-effort: allow X11 connections from the container.
	_ = exec.Command("xhost", "+local:docker").Run()

	if err := runCmd("docker", args...); err != nil {
		ui.Error("failed to start container: " + err.Error())
		os.Exit(1)
	}

	ui.StartDone()
	attach()
}

// attach drops into a shell on the running container. Multiple terminals
// can each call this independently — the container's own zsh (PID 1,
// kept alive by the -dit allocated TTY) stays running regardless of how
// many `docker exec` shells attach or detach.
func attach() {
	cmd := exec.Command("docker", "exec", "-it", config.ContainerName, "zsh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		ui.Error("failed to attach shell: " + err.Error())
	}
}

func Stop() {
	ui.StopHeader()

	if !exists() {
		ui.Warn("container does not exist")
		return
	}

	if err := runCmd("docker", "stop", config.ContainerName); err != nil {
		ui.Error("failed to stop container: " + err.Error())
		os.Exit(1)
	}

	ui.StopDone()
}

func Status() {
	if !exists() {
		ui.Warn("container does not exist — run: kon start")
		return
	}

	state := "stopped"
	if isRunning() {
		state = "running"
	}

	ui.KV("container", config.ContainerName, ui.ClrInfo)
	ui.KV("state", state, statusColor(state))
}

func Logs(follow bool) {
	if !exists() {
		ui.Error("container does not exist — run: kon start")
		os.Exit(1)
	}

	args := []string{"logs"}
	if follow {
		args = append(args, "-f")
	}
	args = append(args, config.ContainerName)

	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

func Exec(args []string) {
	if !isRunning() {
		ui.Error("container is not running — run: kon start")
		os.Exit(1)
	}

	dockerArgs := append([]string{"exec", "-it", config.ContainerName}, args...)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

// ─── Image management ─────────────────────────────────────────────────

// Pull pulls the kon image, streaming layer progress through ui.LayerLine.
func Pull() error {
	cmd := exec.Command("docker", "pull", config.ImageName)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}

	buf := make([]byte, 4096)
	var line []byte
	for {
		n, rerr := stdout.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' || b == '\r' {
					if len(line) > 0 {
						printPullLine(string(line))
						line = line[:0]
					}
					continue
				}
				line = append(line, b)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			break
		}
	}
	if len(line) > 0 {
		printPullLine(string(line))
	}

	return cmd.Wait()
}

func printPullLine(line string) {
	// docker pull output looks like: "<layer id>: <status>"
	id := line
	status := line
	for i := 0; i < len(line); i++ {
		if line[i] == ':' {
			id = line[:i]
			if i+2 <= len(line) {
				status = line[i+2:]
			}
			break
		}
	}
	ui.LayerLine(id, status)
}

// PruneImages removes dangling images and stopped containers tied to kon.
func PruneImages() {
	_ = runCmd("docker", "image", "prune", "-f")
}

// FullCleanup stops/removes the kon container and removes the kon image entirely.
func FullCleanup() {
	if exists() {
		_ = runCmd("docker", "rm", "-f", config.ContainerName)
	}
	_ = runCmd("docker", "rmi", "-f", config.ImageName)
	ui.Ok("removed kon container and image")
}

// ─── Helpers ─────────────────────────────────────────────────────────

func imageExists() bool {
	out, err := exec.Command("docker", "images", "-q", config.ImageName).Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func exists() bool {
	out, err := exec.Command("docker", "ps", "-a", "-q", "-f", "name=^"+config.ContainerName+"$").Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func isRunning() bool {
	out, err := exec.Command("docker", "ps", "-q", "-f", "name=^"+config.ContainerName+"$").Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func statusColor(state string) string {
	if state == "running" {
		return ui.ClrOk
	}
	return ui.ClrWarn
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}

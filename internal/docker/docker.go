// Author: z1rov
package docker

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/z1rov/z1/internal/config"
	"github.com/z1rov/z1/internal/ui"
)

var hostDevices = []string{
	"/dev/net/tun",
}

func Start() {
	ui.StartHeader()

	if !ImageExists() {
		ui.Error("image not found locally — run: z1 install")
		os.Exit(1)
	}

	if IsRunning() {
		ui.Warn("container already running — attaching")
		attach()
		return
	}

	if Exists() {
		_ = exec.Command("docker", "rm", "-f", config.ContainerName).Run()
	}

	if err := os.MkdirAll(config.AnvilDir(), 0755); err != nil {
		ui.Warn("could not create anvil dir: " + err.Error())
	}

	display, xauthPath := resolveX11()

	ui.StartDetail("image", config.ImageName)
	ui.StartDetail("anvil", config.AnvilDir())
	ui.StartDetail("display", display)
	ui.StartDetail("xauth", xauthPath)

	args := []string{
		"run", "-dit",
		"--name", config.ContainerName,
		"--network", "host",
		"--hostname", "z1",
		"--add-host", "z1:127.0.0.1",
		"--user", "root",
		"--cap-add", "SYS_TIME",
		"--cap-add", "NET_ADMIN",
		"--security-opt", "seccomp=unconfined",
		"-e", "DISPLAY=" + display,
		"-e", "XAUTHORITY=/root/.Xauthority",
		"-v", "/tmp/.X11-unix:/tmp/.X11-unix:rw",
		"-v", xauthPath + ":/root/.Xauthority:rw",
		"-v", "/etc/hosts:/etc/hosts",
		"-v", config.AnvilDir() + ":/anvil",
	}

	for _, dev := range resolveDevices() {
		args = append(args, "--device", dev)
		ui.StartDetail("device", dev)
	}

	args = append(args, config.ImageName)

	_ = exec.Command("xhost", "+local:docker").Run()

	if err := runCmd("docker", args...); err != nil {
		ui.Error("failed to start container: " + err.Error())
		os.Exit(1)
	}

	ui.StartDone()
	attach()
}

func resolveDevices() []string {
	var devices []string
	for _, dev := range hostDevices {
		if _, err := os.Stat(dev); err == nil {
			devices = append(devices, dev)
		} else {
			ui.Warn("device not found, skipping: " + dev)
		}
	}
	return devices
}

func resolveX11() (string, string) {
	display := os.Getenv("DISPLAY")
	if display == "" {
		display = ":0"
		ui.Warn("$DISPLAY not set — falling back to :0 (try: sudo -E z1 start)")
	}

	xauth := os.Getenv("XAUTHORITY")
	if xauth == "" {
		home, _ := os.UserHomeDir()
		xauth = home + "/.Xauthority"
	}

	if _, err := os.Stat(xauth); os.IsNotExist(err) {
		ui.Warn("xauth file not found at " + xauth + " — generating a fresh one")
		home, _ := os.UserHomeDir()
		xauth = home + "/.Xauthority"
		_ = exec.Command("touch", xauth).Run()
		if err := exec.Command("xauth", "-f", xauth, "generate", display, ".", "trusted").Run(); err != nil {
			ui.Warn("xauth generate failed: " + err.Error())
		}
	}

	return display, xauth
}

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

	if !Exists() {
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
	if !Exists() {
		ui.Warn("container does not exist — run: z1 start")
		return
	}

	state := "stopped"
	if IsRunning() {
		state = "running"
	}

	ui.KV("container", config.ContainerName, ui.ClrInfo)
	ui.KV("state", state, statusColor(state))
}

func Logs(follow bool) {
	if !Exists() {
		ui.Error("container does not exist — run: z1 start")
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
	if !IsRunning() {
		ui.Error("container is not running — run: z1 start")
		os.Exit(1)
	}

	dockerArgs := append([]string{"exec", "-it", config.ContainerName}, args...)

	cmd := exec.Command("docker", dockerArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()
}

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

func PruneImages() {
	_ = runCmd("docker", "image", "prune", "-f")
}

func FullCleanup() {
	if Exists() {
		_ = runCmd("docker", "rm", "-f", config.ContainerName)
	}
	_ = runCmd("docker", "rmi", "-f", config.ImageName)
	ui.Ok("removed z1 container and image")
}

func ImageExists() bool {
	out, err := exec.Command("docker", "images", "-q", config.ImageName).Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func Exists() bool {
	out, err := exec.Command("docker", "ps", "-a", "-q", "-f", "name=^"+config.ContainerName+"$").Output()
	if err != nil {
		return false
	}
	return len(out) > 0
}

func IsRunning() bool {
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

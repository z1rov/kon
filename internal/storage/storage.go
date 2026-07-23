// Author: z1rov
package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/z1rov/z1/internal/ui"
)

const (
	MinFreeGB     = 40
	DockerSrc     = "/var/lib/docker"
	DaemonJSON    = "/etc/docker/daemon.json"
	dockerDataDir = "docker-data"
	HomeRoot      = "/home"
)

type MountPoint struct {
	Target string
	FreeGB int
	FSType string
}

var skipFSTypes = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "sysfs": true,
	"proc": true, "cgroup": true, "cgroup2": true,
	"overlay": true, "squashfs": true, "devpts": true,
	"fusectl": true, "hugetlbfs": true, "mqueue": true,
}

func EnsureSpace() error {
	currentRoot, err := dockerRoot()
	if err != nil {
		return nil
	}

	freeGB := freeSpaceGB(currentRoot)
	ui.StorageKV("docker root:", currentRoot)
	ui.StorageKV("free space:", fmt.Sprintf("%d GB", freeGB))

	if freeGB >= MinFreeGB {
		ui.StorageOk(fmt.Sprintf("%d GB free - sufficient", freeGB))
		return nil
	}

	return fmt.Errorf("only %d GB free on %s (need %d GB) - run: z1 relocate", freeGB, currentRoot, MinFreeGB)
}

func Relocate() error {
	if os.Getuid() != 0 {
		ui.Banner()
		ui.StorageErr("this command requires root privileges")
		fmt.Printf("\n  %s[Fix]%s Re-run as:\n\n", ui.ClrInfo, ui.ClrReset)
		fmt.Printf("    %ssudo z1 relocate%s\n\n", ui.ClrOk, ui.ClrReset)
		ui.Divider()
		fmt.Println()
		return fmt.Errorf("not running as root")
	}

	ui.Banner()
	fmt.Printf("  %s[Info]%s Docker + containerd relocation\n", ui.ClrInfo, ui.ClrReset)

	_, userName, err := resolveHome()
	if err != nil {
		return fmt.Errorf("could not resolve invoking user: %w", err)
	}

	ui.StorageKV("user:", userName)

	ui.StorageStep("Checking docker group membership...")
	if err := checkDockerGroup(userName); err != nil {
		return err
	}
	ui.StorageOk(fmt.Sprintf("user %q is in the docker group", userName))

	ui.StorageStep("Checking current storage roots...")
	currentDocker, err := dockerRoot()
	if err != nil {
		return fmt.Errorf("Docker daemon not running: %w", err)
	}
	currentContainerd := containerdRoot()

	ui.StorageKV("docker root:", currentDocker)
	ui.StorageKV("containerd root:", currentContainerd)

	dockerSizeGB := dirSizeGB(currentDocker)
	containerdSizeGB := dirSizeGB(currentContainerd)
	needed := dockerSizeGB + containerdSizeGB + 5

	ui.StorageStep("Searching for a suitable target mount...")
	mount, err := bestMount(currentDocker)
	if err != nil {
		return fmt.Errorf("could not find a target location with enough space: %w", err)
	}
	ui.StorageKV("target mount:", mount.Target)
	ui.StorageKV("free on target:", fmt.Sprintf("%d GB", mount.FreeGB))

	if mount.FreeGB < needed {
		return fmt.Errorf("not enough space on %s - need ~%d GB, have %d GB", mount.Target, needed, mount.FreeGB)
	}
	ui.StorageOk(fmt.Sprintf("space check passed (%d GB needed, %d GB available)", needed, mount.FreeGB))

	dstDocker := filepath.Join(mount.Target, dockerDataDir, "docker")
	dstContainerd := filepath.Join(mount.Target, dockerDataDir, "containerd")

	ui.StorageKV("docker target:", dstDocker)
	ui.StorageKV("containerd target:", dstContainerd)

	dockerAlready := currentDocker == dstDocker
	containerdAlready := currentContainerd == dstContainerd

	if dockerAlready && containerdAlready {
		ui.StorageOk("both already at target locations - nothing to do")
		return nil
	}

	ui.StorageKV("docker size:", fmt.Sprintf("~%d GB", dockerSizeGB))
	ui.StorageKV("containerd size:", fmt.Sprintf("~%d GB", containerdSizeGB))

	ui.StorageStep("Stopping Docker services...")
	stopServices()
	ui.StorageOk("services stopped")

	if !dockerAlready {
		ui.StorageStep(fmt.Sprintf("Moving Docker data: %s -> %s", currentDocker, dstDocker))
		if err := moveDir(currentDocker, dstDocker, 0711); err != nil {
			return fmt.Errorf("docker move failed: %w", err)
		}
	} else {
		ui.StorageOk("Docker already at target - skipping")
	}

	ui.StorageStep("Updating /etc/docker/daemon.json...")
	if err := writeDaemonJSON(dstDocker); err != nil {
		return fmt.Errorf("daemon.json: %w", err)
	}
	ui.StorageOk(fmt.Sprintf("data-root -> %s", dstDocker))

	if !containerdAlready {
		ui.StorageStep(fmt.Sprintf("Moving containerd data: %s -> %s", currentContainerd, dstContainerd))
		if err := moveDir(currentContainerd, dstContainerd, 0711); err != nil {
			return fmt.Errorf("containerd move failed: %w", err)
		}
	} else {
		ui.StorageOk("containerd already at target - skipping")
	}

	ui.StorageStep("Updating /etc/containerd/config.toml...")
	if err := writeContainerdConfig(dstContainerd); err != nil {
		return fmt.Errorf("containerd config: %w", err)
	}
	ui.StorageOk(fmt.Sprintf("root -> %s", dstContainerd))

	ui.StorageStep("Restarting Docker + containerd...")
	if err := startServices(); err != nil {
		return fmt.Errorf("restart failed: %w", err)
	}

	actual, err := dockerRoot()
	if err != nil || actual != dstDocker {
		return fmt.Errorf("docker verification failed: expected %s, got %s", dstDocker, actual)
	}
	ui.StorageOk(fmt.Sprintf("relocation complete - Docker root: %s", actual))
	ui.StorageOk(fmt.Sprintf("relocation complete - containerd root: %s", dstContainerd))

	fmt.Println()
	ui.Divider()
	fmt.Println()
	return nil
}

func checkDockerGroup(userName string) error {
	out, err := exec.Command("id", "-nG", userName).Output()
	if err != nil {
		return fmt.Errorf("could not check groups for %s: %w", userName, err)
	}
	groups := strings.Fields(strings.TrimSpace(string(out)))
	for _, g := range groups {
		if g == "docker" {
			return nil
		}
	}

	fmt.Println()
	ui.StorageWarn(fmt.Sprintf("user %q is NOT in the docker group", userName))
	fmt.Printf("\n  %s[Fix]%s Run the following commands:\n\n", ui.ClrInfo, ui.ClrReset)
	fmt.Printf("    %ssudo usermod -aG docker %s%s\n", ui.ClrOk, userName, ui.ClrReset)
	fmt.Printf("    %snewgrp docker%s\n\n", ui.ClrOk, ui.ClrReset)
	fmt.Printf("  %s[*]%s Then log out and back in, or run: %snewgrp docker%s\n\n", ui.ClrDimStr, ui.ClrReset, ui.ClrOk, ui.ClrReset)

	return fmt.Errorf("user %q must be in the docker group before relocating", userName)
}

func resolveHome() (string, string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		u, err := user.Lookup(sudoUser)
		if err == nil && u.HomeDir != "" {
			return u.HomeDir, sudoUser, nil
		}
	}

	u, err := user.Current()
	if err != nil {
		return "", "", fmt.Errorf("cannot determine current user: %w", err)
	}
	if u.HomeDir == "" {
		return "", "", fmt.Errorf("home directory is empty for user %s", u.Username)
	}
	return u.HomeDir, u.Username, nil
}

func dockerRoot() (string, error) {
	out, err := exec.Command("docker", "info", "--format", "{{.DockerRootDir}}").Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", fmt.Errorf("empty DockerRootDir")
	}
	return root, nil
}

func freeSpaceGB(path string) int {
	out, err := exec.Command("df", "-BG", "--output=avail", path).Output()
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return 0
	}
	val := strings.TrimSuffix(strings.TrimSpace(lines[1]), "G")
	n, _ := strconv.Atoi(val)
	return n
}

func dirSizeGB(path string) int {
	out, err := exec.Command("du", "-sB1G", path).Output()
	if err != nil {
		return 0
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	if len(fields) == 0 {
		return 0
	}
	n, _ := strconv.Atoi(fields[0])
	return n
}

func bestMount(currentRoot string) (*MountPoint, error) {
	out, err := exec.Command("df", "-BG", "--output=avail,target").Output()
	if err != nil {
		return nil, fmt.Errorf("df failed: %w", err)
	}

	currentDev := deviceOf(currentRoot)
	var candidates []MountPoint

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Scan()
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		freeStr := strings.TrimSuffix(fields[0], "G")
		target := fields[1]
		free, _ := strconv.Atoi(freeStr)

		if strings.HasPrefix(target, "/boot") ||
			strings.HasPrefix(target, "/sys") ||
			strings.HasPrefix(target, "/proc") ||
			strings.HasPrefix(target, "/dev") ||
			strings.HasPrefix(target, "/run") {
			continue
		}
		if deviceOf(target) == currentDev && currentDev != "" {
			continue
		}
		fstype := fstypeOf(target)
		if skipFSTypes[fstype] {
			continue
		}
		if free >= MinFreeGB {
			candidates = append(candidates, MountPoint{
				Target: target,
				FreeGB: free,
				FSType: fstype,
			})
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no mount point with >=%d GB free found", MinFreeGB)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].FreeGB != candidates[j].FreeGB {
			return candidates[i].FreeGB > candidates[j].FreeGB
		}
		iHome := strings.HasPrefix(candidates[i].Target, "/home")
		jHome := strings.HasPrefix(candidates[j].Target, "/home")
		return iHome && !jHome
	})

	return &candidates[0], nil
}

func deviceOf(path string) string {
	out, err := exec.Command("df", "--output=source", path).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) < 2 {
		return ""
	}
	return strings.TrimSpace(lines[1])
}

func fstypeOf(target string) string {
	out, err := exec.Command("findmnt", "-n", "-o", "FSTYPE", target).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func moveDir(src, dst string, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("mkdir parent %s: %w", filepath.Dir(dst), err)
	}
	srcStat, statErr := os.Stat(src)
	if statErr == nil && srcStat.IsDir() {
		if err := runCmd("cp", "-a", src, dst); err != nil {
			return fmt.Errorf("cp %s -> %s: %w", src, dst, err)
		}
		if err := os.RemoveAll(src); err != nil {
			ui.StorageWarn(fmt.Sprintf("could not remove old dir %s: %v", src, err))
		}
		ui.StorageOk("data moved")
	} else {
		if err := os.MkdirAll(dst, perm); err != nil {
			return fmt.Errorf("mkdir dst %s: %w", dst, err)
		}
		ui.StorageWarn("source was empty - created empty destination")
	}
	return nil
}

const (
	ContainerdSrc = "/var/lib/containerd"
	ContainerdCfg = "/etc/containerd/config.toml"
)

func containerdRoot() string {
	data, err := os.ReadFile(ContainerdCfg)
	if err != nil {
		return ContainerdSrc
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "root") && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"`)
			if val != "" {
				return val
			}
		}
	}
	return ContainerdSrc
}

func writeContainerdConfig(root string) error {
	if err := os.MkdirAll("/etc/containerd", 0755); err != nil {
		return err
	}

	out, err := exec.Command("containerd", "config", "default").Output()
	if err == nil {
		cfg := string(out)
		if strings.Contains(cfg, "root =") {
			cfg = replaceContainerdRoot(cfg, root)
		} else {
			cfg = fmt.Sprintf("root = %q\n", root) + cfg
		}
		return os.WriteFile(ContainerdCfg, []byte(cfg), 0644)
	}

	existing, readErr := os.ReadFile(ContainerdCfg)
	if readErr == nil && strings.Contains(string(existing), "root =") {
		patched := replaceContainerdRoot(string(existing), root)
		return os.WriteFile(ContainerdCfg, []byte(patched), 0644)
	}

	minimal := fmt.Sprintf("root = %q\n", root)
	return os.WriteFile(ContainerdCfg, []byte(minimal), 0644)
}

func replaceContainerdRoot(cfg, root string) string {
	lines := strings.Split(cfg, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "root") && strings.Contains(trimmed, "=") {
			lines[i] = fmt.Sprintf("root = %q", root)
		}
	}
	return strings.Join(lines, "\n")
}

func writeDaemonJSON(dataRoot string) error {
	if err := os.MkdirAll("/etc/docker", 0755); err != nil {
		return err
	}
	raw := map[string]interface{}{}
	if data, err := os.ReadFile(DaemonJSON); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	raw["data-root"] = dataRoot

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(DaemonJSON, out, 0644)
}

func stopServices() {
	_ = runCmd("systemctl", "stop", "docker", "docker.socket", "containerd")
}

func startServices() error {
	if err := runCmd("systemctl", "start", "containerd"); err != nil {
		return err
	}
	return runCmd("systemctl", "start", "docker")
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

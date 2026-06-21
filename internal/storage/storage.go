// Package storage handles Docker data-root migration.
//
// On kon update, if the current Docker data-root partition has less than
// MinFreeGB available, we find the mount point with the most free space,
// move /var/lib/docker there, and update /etc/docker/daemon.json.
package storage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/z1rov/kon/internal/ui"
)

const (
	MinFreeGB     = 40 // minimum free space required on target mount
	DockerSrc     = "/var/lib/docker"
	DaemonJSON    = "/etc/docker/daemon.json"
	dockerDataDir = "docker-data"
)

// MountPoint represents a filesystem mount with free space info.
type MountPoint struct {
	Target string
	FreeGB int
	FSType string
}

// skipFSTypes are virtual / read-only filesystems we never migrate to.
var skipFSTypes = map[string]bool{
	"tmpfs": true, "devtmpfs": true, "sysfs": true,
	"proc": true, "cgroup": true, "cgroup2": true,
	"overlay": true, "squashfs": true, "devpts": true,
	"fusectl": true, "hugetlbfs": true, "mqueue": true,
}

// ─── Public entry point ─────────────────────────────────────────────

// EnsureSpace checks whether the Docker data-root has enough free space for
// a new image pull (estimating the current image size + buffer). If not, it
// migrates Docker to the mount point with the most available space.
//
// Returns nil if everything is fine or migration succeeded.
func EnsureSpace() error {
	currentRoot, err := dockerRoot()
	if err != nil {
		// Docker not running yet — nothing to migrate
		return nil
	}

	freeGB := freeSpaceGB(currentRoot)
	ui.StorageKV("docker root:", currentRoot)
	ui.StorageKV("free space:", fmt.Sprintf("%d GB", freeGB))

	if freeGB >= MinFreeGB {
		ui.StorageOk(fmt.Sprintf("%d GB free — no migration needed", freeGB))
		return nil
	}

	ui.StorageWarn(fmt.Sprintf("only %d GB free (need %d GB) — looking for better mount…", freeGB, MinFreeGB))

	best, err := bestMount(currentRoot)
	if err != nil {
		return fmt.Errorf("could not find a mount with enough space: %w", err)
	}

	ui.StorageKV("target mount:", fmt.Sprintf("%s (%d GB free)", best.Target, best.FreeGB))

	return migrate(currentRoot, best.Target)
}

// ─── Docker root detection ───────────────────────────────────────────

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

// ─── Free space helpers ───────────────────────────────────────────────

// freeSpaceGB returns available GB on the filesystem containing path.
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

// bestMount scans all real mounts and returns the one with the most free space
// (excluding the filesystem that already hosts the Docker root).
func bestMount(currentRoot string) (*MountPoint, error) {
	out, err := exec.Command("df", "-BG", "--output=avail,target").Output()
	if err != nil {
		return nil, fmt.Errorf("df failed: %w", err)
	}

	// Resolve the device of the current root so we skip it.
	currentDev := deviceOf(currentRoot)

	var candidates []MountPoint

	scanner := bufio.NewScanner(bytes.NewReader(out))
	scanner.Scan() // skip header
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		freeStr := strings.TrimSuffix(fields[0], "G")
		target := fields[1]
		free, _ := strconv.Atoi(freeStr)

		// Skip boot, pseudo-fs prefixes
		if strings.HasPrefix(target, "/boot") ||
			strings.HasPrefix(target, "/sys") ||
			strings.HasPrefix(target, "/proc") ||
			strings.HasPrefix(target, "/dev") ||
			strings.HasPrefix(target, "/run") {
			continue
		}

		// Skip same device as current root
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
		return nil, fmt.Errorf("no mount point with ≥%d GB free found", MinFreeGB)
	}

	// Prefer /home over others if tied, otherwise just take most free.
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].FreeGB != candidates[j].FreeGB {
			return candidates[i].FreeGB > candidates[j].FreeGB
		}
		// tiebreak: /home preferred
		iHome := strings.HasPrefix(candidates[i].Target, "/home")
		jHome := strings.HasPrefix(candidates[j].Target, "/home")
		return iHome && !jHome
	})

	return &candidates[0], nil
}

// deviceOf returns the block device for a given path (empty string on error).
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

// fstypeOf returns the filesystem type of a mount point.
func fstypeOf(target string) string {
	out, err := exec.Command("findmnt", "-n", "-o", "FSTYPE", target).Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// ─── Migration ───────────────────────────────────────────────────────

func migrate(currentRoot, targetMount string) error {
	dst := filepath.Join(targetMount, dockerDataDir, "docker")

	// If destination already exists and daemon.json already points there, done.
	if currentRoot == dst {
		ui.StorageOk("Docker already on target mount — nothing to do")
		return nil
	}

	ui.StorageStep("Stopping Docker services…")
	stopServices()

	// Move data
	ui.StorageStep(fmt.Sprintf("Moving Docker data: %s → %s", currentRoot, dst))
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(dst), err)
	}

	srcStat, err := os.Stat(currentRoot)
	if err == nil && srcStat.IsDir() {
		if err := runCmd("cp", "-a", currentRoot, dst); err != nil {
			return fmt.Errorf("cp failed: %w", err)
		}
		if err := os.RemoveAll(currentRoot); err != nil {
			ui.StorageWarn(fmt.Sprintf("could not remove old root %s: %v", currentRoot, err))
		}
		ui.StorageOk("data moved")
	} else {
		// Nothing to move — just create the empty dir
		if err := os.MkdirAll(dst, 0711); err != nil {
			return fmt.Errorf("mkdir dst: %w", err)
		}
		ui.StorageWarn("source was empty — created empty destination")
	}

	// Write daemon.json
	ui.StorageStep("Updating /etc/docker/daemon.json…")
	if err := writeDaemonJSON(dst); err != nil {
		return fmt.Errorf("daemon.json: %w", err)
	}
	ui.StorageOk(fmt.Sprintf("data-root set to %s", dst))

	// Restart
	ui.StorageStep("Restarting Docker…")
	if err := startServices(); err != nil {
		return fmt.Errorf("docker restart failed: %w", err)
	}

	// Verify
	actual, err := dockerRoot()
	if err != nil || actual != dst {
		return fmt.Errorf("verification failed: expected %s, got %s", dst, actual)
	}
	ui.StorageOk(fmt.Sprintf("migration complete — Docker root: %s", actual))
	return nil
}

// ─── daemon.json ─────────────────────────────────────────────────────

type daemonConfig struct {
	DataRoot string `json:"data-root"`
}

func writeDaemonJSON(dataRoot string) error {
	if err := os.MkdirAll("/etc/docker", 0755); err != nil {
		return err
	}

	// Merge with existing config if present (preserve other keys).
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

// ─── Service control ─────────────────────────────────────────────────

func stopServices() {
	_ = runCmd("systemctl", "stop", "docker", "docker.socket", "containerd")
}

func startServices() error {
	if err := runCmd("systemctl", "start", "containerd"); err != nil {
		return err
	}
	return runCmd("systemctl", "start", "docker")
}

// ─── Util ────────────────────────────────────────────────────────────

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

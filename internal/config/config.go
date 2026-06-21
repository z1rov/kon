package config

import "os"

const (
	// Image
	ImageName     = "ghcr.io/z1rov/kon-image:latest"
	ContainerName = "kon"

	// Remote — version file
	VersionURL = "https://raw.githubusercontent.com/z1rov/kon-images/refs/heads/main/version/version.txt"

	// Version parsing — expects X.X
	VersionRegex = `^([\d]+\.[\d]+)`
)

// AnvilDir returns the host path mounted into the container as /anvil.
// Lives under the user's home (~/anvil) instead of system root (/anvil)
// to avoid root-owned-directory permission headaches.
func AnvilDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		// Fallback if HOME can't be resolved for some reason.
		return "/anvil"
	}
	return home + "/anvil"
}

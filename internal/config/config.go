package config

const (
	// Image
	ImageName     = "ghcr.io/z1rov/kon-image:latest"
	ContainerName = "kon"

	// Remote - Version file (solo contiene "1.0")
	VersionURL = "https://raw.githubusercontent.com/z1rov/kon-images/refs/heads/main/version/version.txt"

	// Version parsing - Ahora espera solo X.X
	VersionRegex = `^([\d]+\.[\d]+)`
)

// AnvilDir returns /anvil (mount point inside container)
func AnvilDir() string {
	return "/anvil"
}

package config

import "os"

const (
	ImageName     = "ghcr.io/z1rov/kon-image:latest"
	ContainerName = "kon"

	VersionURL = "https://raw.githubusercontent.com/z1rov/kon-images/refs/heads/main/version/version.txt"

	VersionRegex = `^([\d]+\.[\d]+)`
)

func AnvilDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/anvil"
	}
	return home + "/anvil"
}

// Author: z1rov
package config

import "os"

const (
	ImageName     = "ghcr.io/z1rov/z1-images/z1-images:latest"
	ContainerName = "z1"

	VersionURL = "https://raw.githubusercontent.com/z1rov/z1-images/refs/heads/main/version/version.txt"

	VersionRegex = `^([\d]+\.[\d]+)`
)

func AnvilDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "/anvil"
	}
	return home + "/anvil"
}

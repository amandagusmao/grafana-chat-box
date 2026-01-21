//go:build mage
// +build mage

package main

import (
	"github.com/magefile/mage/sh"
)

// BuildAll builds the backend for all platforms
func BuildAll() error {
	if err := buildBackend("linux", "amd64"); err != nil {
		return err
	}
	if err := buildBackend("linux", "arm64"); err != nil {
		return err
	}
	if err := buildBackend("windows", "amd64"); err != nil {
		return err
	}
	if err := buildBackend("darwin", "amd64"); err != nil {
		return err
	}
	return buildBackend("darwin", "arm64")
}

// BuildLinux builds for Linux amd64
func BuildLinux() error {
	return buildBackend("linux", "amd64")
}

// BuildWindows builds for Windows amd64
func BuildWindows() error {
	return buildBackend("windows", "amd64")
}

// BuildDarwin builds for macOS (amd64 and arm64)
func BuildDarwin() error {
	if err := buildBackend("darwin", "amd64"); err != nil {
		return err
	}
	return buildBackend("darwin", "arm64")
}

func buildBackend(goos, goarch string) error {
	env := map[string]string{
		"GOOS":        goos,
		"GOARCH":      goarch,
		"CGO_ENABLED": "0",
	}

	ext := ""
	if goos == "windows" {
		ext = ".exe"
	}

	outputPath := "dist/gpx_grafana-sqlserver-datasource_" + goos + "_" + goarch + ext
	return sh.RunWithV(env, "go", "build", "-o", outputPath, "-ldflags", "-w -s", "./pkg")
}

// Clean removes build artifacts
func Clean() error {
	return sh.Rm("dist")
}

// Test runs Go tests
func Test() error {
	return sh.RunV("go", "test", "./pkg/...")
}

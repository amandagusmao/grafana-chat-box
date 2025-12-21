//go:build mage
// +build mage

package main

import (
	"github.com/magefile/mage/sh"
)

// Build builds the plugin backend for multiple platforms
func Build() error {
	return sh.RunV("mage", "-v", "buildAll")
}

// BuildAll builds the backend plugin for all supported platforms
func BuildAll() error {
	// Build for Linux (amd64)
	if err := buildBackend("linux", "amd64"); err != nil {
		return err
	}
	// Build for Linux (arm64)
	if err := buildBackend("linux", "arm64"); err != nil {
		return err
	}
	// Build for Windows (amd64)
	if err := buildBackend("windows", "amd64"); err != nil {
		return err
	}
	// Build for Darwin/macOS (amd64)
	if err := buildBackend("darwin", "amd64"); err != nil {
		return err
	}
	// Build for Darwin/macOS (arm64)
	if err := buildBackend("darwin", "arm64"); err != nil {
		return err
	}
	return nil
}

// BuildLinux builds the backend plugin for Linux only
func BuildLinux() error {
	return buildBackend("linux", "amd64")
}

// BuildWindows builds the backend plugin for Windows only
func BuildWindows() error {
	return buildBackend("windows", "amd64")
}

// BuildDarwin builds the backend plugin for macOS only
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

	outputPath := "dist/gpx_grafana-chat-assistant_" + goos + "_" + goarch + ext

	return sh.RunWithV(env, "go", "build", "-o", outputPath, "-ldflags", "-w -s", "./pkg")
}

// Clean removes build artifacts
func Clean() error {
	return sh.Rm("dist")
}

// Test runs the Go tests
func Test() error {
	return sh.RunV("go", "test", "./pkg/...")
}

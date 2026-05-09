package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type updateManifest struct {
	Version      string                         `json:"version"`
	ReleaseDate  string                         `json:"release_date"`
	ReleaseNotes string                         `json:"release_notes"`
	Platforms    map[string]updateManifestAsset `json:"platforms"`
	Mandatory    bool                           `json:"mandatory"`
}

type updateManifestAsset struct {
	URL      string `json:"url"`
	Size     int64  `json:"size"`
	Checksum string `json:"checksum"`
}

type assetSpec struct {
	platform string
	suffix   string
}

var releaseAssets = []assetSpec{
	{platform: "macos-arm64", suffix: ".tar.gz"},
	{platform: "macos-amd64", suffix: ".tar.gz"},
	{platform: "windows-amd64", suffix: ".zip"},
}

func main() {
	if len(os.Args) < 2 {
		exitf("usage: go run ./scripts/release <manifest> [flags]")
	}

	switch os.Args[1] {
	case "manifest":
		runManifest(os.Args[2:])
	default:
		exitf("unknown subcommand: %s", os.Args[1])
	}
}

func runManifest(args []string) {
	flags := flag.NewFlagSet("manifest", flag.ExitOnError)
	version := flags.String("version", "", "release version")
	assetsDir := flags.String("assets-dir", "", "directory containing release assets")
	outputPath := flags.String("out", "", "manifest output file")
	repo := flags.String("repo", os.Getenv("APP_REPOSITORY"), "GitHub repo in owner/repo form")
	baseName := flags.String("base-name", envOrDefault("APP_BINARY_NAME", "minidb"), "release asset basename")
	notesPath := flags.String("notes", "", "optional release notes markdown file")
	mandatory := flags.Bool("mandatory", false, "mark update as mandatory")
	_ = flags.Parse(args)

	resolvedVersion := strings.TrimSpace(strings.TrimPrefix(*version, "v"))
	if resolvedVersion == "" {
		exitf("version is required")
	}
	if strings.TrimSpace(*assetsDir) == "" {
		exitf("assets-dir is required")
	}
	if strings.TrimSpace(*outputPath) == "" {
		exitf("manifest output path is required")
	}
	if strings.TrimSpace(*repo) == "" {
		exitf("repo is required")
	}

	notes, err := resolveReleaseNotes(*notesPath)
	if err != nil {
		exitErr(err)
	}

	manifest := updateManifest{
		Version:      resolvedVersion,
		ReleaseDate:  time.Now().UTC().Format(time.RFC3339),
		ReleaseNotes: notes,
		Platforms:    map[string]updateManifestAsset{},
		Mandatory:    *mandatory,
	}

	for _, spec := range releaseAssets {
		filename := fmt.Sprintf("%s-%s-%s%s", *baseName, resolvedVersion, spec.platform, spec.suffix)
		fullpath := filepath.Join(*assetsDir, filename)
		asset, err := buildManifestAsset(fullpath, *repo, resolvedVersion, filename)
		if err != nil {
			exitErr(err)
		}
		manifest.Platforms[spec.platform] = asset
	}

	if err := os.MkdirAll(filepath.Dir(*outputPath), 0o755); err != nil {
		exitErr(err)
	}
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		exitErr(err)
	}
	content = append(content, '\n')
	if err := os.WriteFile(*outputPath, content, 0o644); err != nil {
		exitErr(err)
	}
}

func resolveReleaseNotes(sourcePath string) (string, error) {
	sourcePath = strings.TrimSpace(sourcePath)
	if sourcePath == "" {
		return "See GitHub Releases for details.", nil
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}
	notes := strings.TrimSpace(string(content))
	if notes == "" {
		return "", errors.New("release notes file is empty")
	}
	return notes, nil
}

func buildManifestAsset(path, repo, version, filename string) (updateManifestAsset, error) {
	info, err := os.Stat(path)
	if err != nil {
		return updateManifestAsset{}, err
	}

	checksum, err := sha256File(path)
	if err != nil {
		return updateManifestAsset{}, err
	}

	return updateManifestAsset{
		URL:      fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, version, filename),
		Size:     info.Size(),
		Checksum: "sha256:" + checksum,
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func sha256File(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func exitErr(err error) {
	exitf("%v", err)
}

func exitf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

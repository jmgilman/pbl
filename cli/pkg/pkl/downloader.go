package pkl

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/spf13/afero"
)

// HTTPClient defines the interface for making HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// Runtime provides runtime information
type Runtime interface {
	GOOS() string
	GOARCH() string
}

// defaultRuntime implements Runtime using the standard runtime package
type defaultRuntime struct{}

func (d *defaultRuntime) GOOS() string {
	return runtime.GOOS
}

func (d *defaultRuntime) GOARCH() string {
	return runtime.GOARCH
}

type PklDownloader struct {
	logger     *slog.Logger
	httpClient HTTPClient
	fs         afero.Fs
	runtime    Runtime
}

// Option is a function that configures a PklDownloader
type Option func(*PklDownloader)

// WithLogger sets the logger for the PklDownloader
func WithLogger(logger *slog.Logger) Option {
	return func(d *PklDownloader) {
		d.logger = logger
	}
}

// WithHTTPClient sets the HTTP client for the PklDownloader
func WithHTTPClient(client HTTPClient) Option {
	return func(d *PklDownloader) {
		d.httpClient = client
	}
}

// WithFilesystem sets the filesystem for the PklDownloader
func WithFilesystem(fs afero.Fs) Option {
	return func(d *PklDownloader) {
		d.fs = fs
	}
}

// WithRuntime sets the runtime for the PklDownloader
func WithRuntime(rt Runtime) Option {
	return func(d *PklDownloader) {
		d.runtime = rt
	}
}

// NewPklDownloader creates a new PklDownloader instance with the provided options
func NewPklDownloader(opts ...Option) *PklDownloader {
	d := &PklDownloader{
		logger:     slog.Default(),
		httpClient: &http.Client{Timeout: 10 * time.Second},
		fs:         afero.NewOsFs(),
		runtime:    &defaultRuntime{},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

// Download fetches the latest version of Pkl and saves it to the specified path
func (d *PklDownloader) Download(path string) error {
	d.logger.Info("Starting Pkl download", "path", path)

	version, err := d.getLatestPklVersion()
	if err != nil {
		return fmt.Errorf("failed to get latest version: %w", err)
	}

	downloadURL, err := d.getPklDownloadURL(version)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	d.logger.Debug("Downloading Pkl binary", "version", version, "url", downloadURL)
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create download request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download binary: status code %d", resp.StatusCode)
	}

	// Create the file
	out, err := d.fs.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Make the file executable
	if err := d.fs.Chmod(path, 0755); err != nil {
		return fmt.Errorf("failed to make file executable: %w", err)
	}

	d.logger.Info("Successfully downloaded Pkl", "version", version, "path", path)
	return nil
}

// getLatestPklVersion fetches the latest release version of Pkl from GitHub
func (d *PklDownloader) getLatestPklVersion() (string, error) {
	d.logger.Debug("Fetching latest Pkl version from GitHub")
	req, err := http.NewRequest("GET", "https://api.github.com/repos/apple/pkl/releases/latest", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add GitHub API version header
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch latest release: status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &release); err != nil {
		return "", fmt.Errorf("failed to parse release data: %w", err)
	}

	d.logger.Debug("Found latest Pkl version", "version", release.TagName)
	return release.TagName, nil
}

// getPklDownloadURL returns the download URL for the latest Pkl release
func (d *PklDownloader) getPklDownloadURL(version string) (string, error) {
	baseURL := fmt.Sprintf("https://github.com/apple/pkl/releases/download/%s/", version)

	downloadMap := map[string]string{
		"darwin/amd64":  "pkl-macos-amd64",
		"darwin/arm64":  "pkl-macos-aarch64", // Note: Go arm64 -> pkl aarch64
		"linux/amd64":   "pkl-linux-amd64",
		"linux/arm64":   "pkl-linux-aarch64", // Note: Go arm64 -> pkl aarch64
		"windows/amd64": "pkl-windows-amd64.exe",
	}

	goos := d.runtime.GOOS()
	goarch := d.runtime.GOARCH()
	lookupKey := fmt.Sprintf("%s/%s", goos, goarch)

	filename, supported := downloadMap[lookupKey]
	if !supported {
		return "", fmt.Errorf("unsupported OS/architecture combination: %s/%s", goos, goarch)
	}

	return baseURL + filename, nil
}

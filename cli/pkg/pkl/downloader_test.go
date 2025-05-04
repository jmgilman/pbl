package pkl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// mockRuntime is a test helper that provides runtime information
type mockRuntime struct {
	goos   string
	goarch string
}

func (m *mockRuntime) GOOS() string {
	return m.goos
}

func (m *mockRuntime) GOARCH() string {
	return m.goarch
}

func TestPklDownloader_Download(t *testing.T) {
	type testCase struct {
		name        string
		goos        string
		goarch      string
		version     string
		downloadURL string
	}

	successCases := []testCase{
		{
			name:        "darwin/amd64",
			goos:        "darwin",
			goarch:      "amd64",
			version:     "0.28.2",
			downloadURL: "https://github.com/apple/pkl/releases/download/0.28.2/pkl-macos-amd64",
		},
		{
			name:        "darwin/arm64",
			goos:        "darwin",
			goarch:      "arm64",
			version:     "0.28.2",
			downloadURL: "https://github.com/apple/pkl/releases/download/0.28.2/pkl-macos-aarch64",
		},
		{
			name:        "linux/amd64",
			goos:        "linux",
			goarch:      "amd64",
			version:     "0.28.2",
			downloadURL: "https://github.com/apple/pkl/releases/download/0.28.2/pkl-linux-amd64",
		},
		{
			name:        "linux/arm64",
			goos:        "linux",
			goarch:      "arm64",
			version:     "0.28.2",
			downloadURL: "https://github.com/apple/pkl/releases/download/0.28.2/pkl-linux-aarch64",
		},
		{
			name:        "windows/amd64",
			goos:        "windows",
			goarch:      "amd64",
			version:     "0.28.2",
			downloadURL: "https://github.com/apple/pkl/releases/download/0.28.2/pkl-windows-amd64.exe",
		},
	}

	// Failure test cases
	failureCases := []struct {
		name        string
		goos        string
		goarch      string
		version     string
		downloadURL string
		setup       func(t *testing.T, mockClient *MockHTTPClient, memFs afero.Fs)
		expectedErr string
	}{
		{
			name:    "unsupported platform",
			goos:    "unsupported",
			goarch:  "unsupported",
			version: "0.28.2",
			setup: func(t *testing.T, mockClient *MockHTTPClient, memFs afero.Fs) {
				releaseJSON := []byte(`{"tag_name": "0.28.2"}`)
				mockClient.EXPECT().
					Do(mock.MatchedBy(func(req *http.Request) bool {
						return req.URL.String() == "https://api.github.com/repos/apple/pkl/releases/latest"
					})).
					Return(&http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(releaseJSON)),
					}, nil)
			},
			expectedErr: "failed to get download URL: unsupported OS/architecture combination: unsupported/unsupported",
		},
		{
			name:    "github API error",
			goos:    "linux",
			goarch:  "amd64",
			version: "0.28.2",
			setup: func(t *testing.T, mockClient *MockHTTPClient, memFs afero.Fs) {
				mockClient.EXPECT().
					Do(mock.MatchedBy(func(req *http.Request) bool {
						return req.URL.String() == "https://api.github.com/repos/apple/pkl/releases/latest"
					})).
					Return(nil, fmt.Errorf("connection refused"))
			},
			expectedErr: "failed to get latest version: failed to fetch latest release: connection refused",
		},
		{
			name:    "github API non-200 response",
			goos:    "linux",
			goarch:  "amd64",
			version: "0.28.2",
			setup: func(t *testing.T, mockClient *MockHTTPClient, memFs afero.Fs) {
				mockClient.EXPECT().
					Do(mock.MatchedBy(func(req *http.Request) bool {
						return req.URL.String() == "https://api.github.com/repos/apple/pkl/releases/latest"
					})).
					Return(&http.Response{
						StatusCode: http.StatusNotFound,
						Body:       io.NopCloser(bytes.NewReader([]byte("Not Found"))),
					}, nil)
			},
			expectedErr: "failed to get latest version: failed to fetch latest release: status code 404",
		},
		{
			name:        "download error",
			goos:        "linux",
			goarch:      "amd64",
			version:     "0.28.2",
			downloadURL: "https://github.com/apple/pkl/releases/download/0.28.2/pkl-linux-amd64",
			setup: func(t *testing.T, mockClient *MockHTTPClient, memFs afero.Fs) {
				releaseJSON := []byte(`{"tag_name": "0.28.2"}`)
				mockClient.EXPECT().
					Do(mock.MatchedBy(func(req *http.Request) bool {
						return req.URL.String() == "https://api.github.com/repos/apple/pkl/releases/latest"
					})).
					Return(&http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader(releaseJSON)),
					}, nil)

				mockClient.EXPECT().
					Do(mock.MatchedBy(func(req *http.Request) bool {
						return req.URL.String() == "https://github.com/apple/pkl/releases/download/0.28.2/pkl-linux-amd64"
					})).
					Return(nil, fmt.Errorf("connection refused"))
			},
			expectedErr: "failed to download binary: connection refused",
		},
	}

	for _, tc := range successCases {
		t.Run("success/"+tc.name, func(t *testing.T) {
			// Setup
			mockClient := NewMockHTTPClient(t)
			memFs := afero.NewMemMapFs()
			mockRT := &mockRuntime{
				goos:   tc.goos,
				goarch: tc.goarch,
			}

			downloader := NewPklDownloader(
				WithHTTPClient(mockClient),
				WithFilesystem(memFs),
				WithRuntime(mockRT),
			)

			// Mock GitHub API response
			releaseJSON := []byte(fmt.Sprintf(`{"tag_name": "%s"}`, tc.version))
			mockClient.EXPECT().
				Do(mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == "https://api.github.com/repos/apple/pkl/releases/latest"
				})).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(releaseJSON)),
				}, nil)

			// Mock binary download response
			mockClient.EXPECT().
				Do(mock.MatchedBy(func(req *http.Request) bool {
					return req.URL.String() == tc.downloadURL
				})).
				Return(&http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(make([]byte, 10))), // 10 bytes of dummy data
				}, nil)

			// Execute
			path := "/tmp/pkl"
			err := downloader.Download(path)
			require.NoError(t, err)

			// Validate
			exists, err := afero.Exists(memFs, path)
			require.NoError(t, err)
			assert.True(t, exists)

			info, err := memFs.Stat(path)
			require.NoError(t, err)
			assert.Equal(t, int64(10), info.Size())

			// On Windows, we only check if the file is executable
			// On Unix-like systems, we check the exact mode
			if tc.goos == "windows" {
				assert.True(t, info.Mode()&0111 != 0, "file should be executable")
			} else {
				// Use os.FileMode to mask out any additional bits
				assert.Equal(t, os.FileMode(0755), info.Mode()&os.ModePerm)
			}
		})
	}

	for _, tc := range failureCases {
		t.Run("failure/"+tc.name, func(t *testing.T) {
			// Setup
			mockClient := NewMockHTTPClient(t)
			memFs := afero.NewMemMapFs()
			mockRT := &mockRuntime{
				goos:   tc.goos,
				goarch: tc.goarch,
			}

			downloader := NewPklDownloader(
				WithHTTPClient(mockClient),
				WithFilesystem(memFs),
				WithRuntime(mockRT),
			)

			// Setup test case
			tc.setup(t, mockClient, memFs)

			// Execute
			path := "/tmp/pkl"
			err := downloader.Download(path)

			// Validate
			require.Error(t, err)
			assert.Equal(t, tc.expectedErr, err.Error())
		})
	}
}

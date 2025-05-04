package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/jmgilman/pbl/cli/internal/run"
	"github.com/jmgilman/pbl/cli/pkg/pkl"
	"github.com/jmgilman/pbl/schema"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
)

// version holds the version of the CLI application.
var version = "dev"

// GlobalArgs holds the global command-line arguments.
type GlobalArgs struct {
	Test    TestCmd `cmd:"" help:"Test command."`
	Verbose int     `short:"v" type:"counter" help:"Enable verbose logging."`
}

// cli is the main command-line interface structure.
var cli struct {
	GlobalArgs

	Version VersionCmd `cmd:"" help:"Print the version."`

	ShellCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
}

type TestCmd struct{}

func (c *TestCmd) Run(ctx run.RunContext) error {
	cfg, err := schema.LoadFromPath(context.Background(), "test.pkl")
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	fmt.Printf("Got name: %s", cfg.Project.Name)

	return nil
}

// VersionCmd is the command to print the version of the CLI application.
type VersionCmd struct{}

func (c *VersionCmd) Run(ctx run.RunContext) error {
	fmt.Printf("pbl version %s %s/%s\n", version, runtime.GOOS, runtime.GOARCH)
	return nil
}

// Run is the main entry point for the CLI application.
func Run() int {
	cliArgs := os.Args[1:]

	parser := kong.Must(&cli,
		kong.Name("pbl"),
		kong.Description("The PicoBlade CLI tool"))

	kongplete.Complete(parser,
		kongplete.WithPredictor("path", complete.PredictFiles("*")),
	)

	ctx, err := parser.Parse(cliArgs)
	if err != nil {
		printAndExit(err)
	}

	handler := log.New(os.Stderr)
	switch cli.Verbose {
	case 0:
		handler.SetLevel(log.FatalLevel)
	case 1:
		handler.SetLevel(log.WarnLevel)
	case 2:
		handler.SetLevel(log.InfoLevel)
	case 3:
		handler.SetLevel(log.DebugLevel)
	}

	logger := slog.New(handler)
	rtx := run.RunContext{
		Logger: logger,
	}
	ctx.Bind(rtx)

	pklPath, err := checkPklBinary(logger)
	if err != nil {
		printAndExit(err)
	}
	logger.Debug("Using pkl binary", "path", pklPath)

	if err := ctx.Run(); err != nil {
		printAndExit(err)
	}

	return 0
}

// getInstallPath returns the appropriate installation path for the pkl binary based on the OS
func getInstallPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create .local/bin directory: %w", err)
	}

	binaryName := "pkl"
	if runtime.GOOS == "windows" {
		binaryName = "pkl.exe"
	}
	return filepath.Join(binDir, binaryName), nil
}

// checkPklBinary checks if the pkl binary is available in PATH and prompts for installation if not found.
// Returns the path to the pkl binary and any error that occurred.
func checkPklBinary(logger *slog.Logger) (string, error) {
	pklPath, err := exec.LookPath("pkl")
	if err != nil {
		logger.Error("pkl binary not found in PATH")

		var shouldInstall bool
		form := huh.NewForm(
			huh.NewGroup(
				huh.NewConfirm().
					Title("pkl binary not found in PATH").
					Description("Would you like to install pkl now?").
					Value(&shouldInstall),
			),
		)

		if err := form.Run(); err != nil {
			return "", fmt.Errorf("failed to show installation prompt: %w", err)
		}

		if !shouldInstall {
			return "", fmt.Errorf("pkl binary not found in PATH. Please install pkl first")
		}

		installPath, err := getInstallPath()
		if err != nil {
			return "", fmt.Errorf("failed to determine installation path: %w", err)
		}

		downloader := pkl.NewPklDownloader(
			pkl.WithLogger(logger),
		)

		logger.Info("Downloading pkl...")
		if err := downloader.Download(installPath); err != nil {
			return "", fmt.Errorf("failed to download pkl: %w", err)
		}

		logger.Info("Successfully installed pkl", "path", installPath)
		return installPath, nil
	}

	logger.Debug("Found pkl binary", "path", pklPath)
	return pklPath, nil
}

func main() {
	os.Exit(Run())
}

// printAndExit prints the error message to stderr and exits with a non-zero status code.
func printAndExit(err error) {
	fmt.Fprintf(os.Stderr, "pbl: %v\n", err)
	os.Exit(1)
}

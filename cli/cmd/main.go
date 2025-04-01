package main

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/alecthomas/kong"
	"github.com/charmbracelet/log"
	"github.com/jmgilman/pbl/cli/internal/run"
	"github.com/posener/complete"
	"github.com/willabides/kongplete"
)

// version holds the version of the CLI application.
var version = "dev"

// GlobalArgs holds the global command-line arguments.
type GlobalArgs struct {
	Verbose int `short:"v" type:"counter" help:"Enable verbose logging."`
}

// cli is the main command-line interface structure.
var cli struct {
	GlobalArgs

	Version VersionCmd `cmd:"" help:"Print the version."`

	ShellCompletions kongplete.InstallCompletions `cmd:"" help:"Install shell completions"`
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

	if err := ctx.Run(); err != nil {
		printAndExit(err)
	}

	return 0
}

func main() {
	os.Exit(Run())
}

// printAndExit prints the error message to stderr and exits with a non-zero status code.
func printAndExit(err error) {
	fmt.Fprintf(os.Stderr, "pbl: %v\n", err)
	os.Exit(1)
}

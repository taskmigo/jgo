package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	gots "github.com/example/gots"
	"io"
	"os"
	"time"
)

type cliConfig struct {
	expression   string
	filename     string
	maxSteps     uint64
	maxCallDepth int
	timeout      time.Duration
}

func runCLI(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	config, err := parseCLIConfig(args, stderr)
	if errors.Is(err, flag.ErrHelp) {
		return 0
	}
	if err != nil {
		return 2
	}

	source, err := readSource(config, stdin)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	runtime := gots.New(gots.WithMaxSteps(config.maxSteps), gots.WithMaxCallDepth(config.maxCallDepth))
	ctx := context.Background()
	var cancel context.CancelFunc
	if config.timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, config.timeout)
		defer cancel()
	}
	value, err := runtime.RunStringContext(ctx, string(source))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	fmt.Fprintln(stdout, value.String())
	return 0
}

func parseCLIConfig(args []string, stderr io.Writer) (cliConfig, error) {
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)

	var config cliConfig
	flags.StringVar(&config.expression, "e", "", "script expression")
	flags.Uint64Var(&config.maxSteps, "steps", 0, "maximum interpreter steps")
	flags.IntVar(&config.maxCallDepth, "call-depth", 256, "maximum call depth")
	flags.DurationVar(&config.timeout, "timeout", 0, "execution timeout")
	if err := flags.Parse(args); err != nil {
		return cliConfig{}, err
	}
	if flags.NArg() > 0 {
		config.filename = flags.Arg(0)
	}
	return config, nil
}

func readSource(config cliConfig, stdin io.Reader) ([]byte, error) {
	if config.expression != "" {
		return []byte(config.expression), nil
	}
	if config.filename != "" {
		return os.ReadFile(config.filename)
	}
	return io.ReadAll(stdin)
}

package main

import (
	"context"
	"flag"
	"fmt"
	gots "github.com/example/gots"
	"io"
	"os"
	"time"
)

func main() {
	expr := flag.String("e", "", "script expression")
	steps := flag.Uint64("steps", 0, "maximum interpreter steps")
	depth := flag.Int("call-depth", 256, "maximum call depth")
	timeout := flag.Duration("timeout", 0, "execution timeout")
	flag.Parse()
	var src []byte
	var err error
	if *expr != "" {
		src = []byte(*expr)
	} else if flag.NArg() > 0 {
		src, err = os.ReadFile(flag.Arg(0))
	} else {
		src, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	r := gots.New(gots.WithMaxSteps(*steps), gots.WithMaxCallDepth(*depth))
	ctx := context.Background()
	var cancel context.CancelFunc
	if *timeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, *timeout)
		defer cancel()
	}
	v, err := r.RunStringContext(ctx, string(src))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(v.String())
	_ = time.Second
}

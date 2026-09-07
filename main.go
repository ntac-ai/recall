package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	// Keep the default signal handling so Ctrl+C also interrupts blocking stdin
	// reads. File publication never exposes a partially written memory or skill.
	if err := run(context.Background(), os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "recall:", err)
		os.Exit(1)
	}
}

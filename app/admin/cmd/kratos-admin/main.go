package main

import (
	"context"
	"fmt"
	"os"
)

func main() {
	runners := commandRunners{
		server:    runServer,
		worker:    runWorker,
		initAdmin: runInitAdmin,
		gormGen:   runGORMGen,
	}
	if err := newRootCommand(runners).ExecuteContext(context.Background()); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

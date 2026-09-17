package main

import (
	"context"
	"cosmoralabs/nearprod/internal/nearprod"
	"cosmoralabs/nearprod/internal/webui"
	"io/fs"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	assets, _ := fs.Sub(webui.Files, "dist")
	os.Exit(nearprod.RunCLI(ctx, os.Args[1:], assets, os.Stdout, os.Stderr))
}

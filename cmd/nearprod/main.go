package main

import (
	"context"
	"io/fs"
	"nearprod/internal/nearprod"
	"nearprod/internal/webui"
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

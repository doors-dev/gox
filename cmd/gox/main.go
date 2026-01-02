package main

//go:generate cargo build --release --manifest-path=../../formatter/Cargo.toml

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/doors-dev/gox/cmd/gox/command"
)

func main() {
	initLogger()
	cmdErr, runErr := command.Execute(starter{})
	if cmdErr != nil {
		fmt.Fprintln(os.Stderr, cmdErr.Error())
		os.Exit(1)
		return
	}
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr.Error())
		os.Exit(1)
		return
	}
}

func initLogger() {
	f, err := os.OpenFile("/tmp/gox.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		panic(err)
	}
	h := slog.NewTextHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug, // or LevelDebug, LevelWarn, LevelError
	})
	slog.SetDefault(slog.New(h))
}

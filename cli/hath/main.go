package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	hServer "github.com/mayocream/hath-go/server"
	fiber "github.com/mayocream/hath-go/server/fiber"
	"github.com/spf13/pflag"
)

var cfgFile = pflag.StringP("config", "f", "", "config file")

func main() {
	pflag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := parseCfg(*cfgFile)
	if err != nil {
		exit(err)
	}

	h, err := hServer.NewHath(*cfg)
	if err != nil {
		exit(err)
	}

	s := fiber.NewServer(h)
	s.Serve(ctx)
}

func exit(err error) {
	fmt.Fprint(os.Stderr, err)
	os.Exit(1)
}
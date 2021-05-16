package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	hServer "github.com/mayocream/hath-go/server"
	fiber "github.com/mayocream/hath-go/server/fiber"
	"github.com/pkg/errors"
	"github.com/spf13/pflag"
	"go.uber.org/zap"
)

var cfgFile = pflag.StringP("config", "f", "", "config file")

func main() {
	godotenv.Load()
	pflag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := parseCfg(*cfgFile)
	if err != nil {
		exit(errors.Wrap(err, "load config"))
	}

	h, err := hServer.NewHath(*cfg)
	if err != nil {
		exit(errors.Wrap(err, "init hath server"))
	}

	wg := &sync.WaitGroup{}

	s := fiber.NewServer(h)
	
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := s.Serve(ctx); err != nil {
			exit(err)
		}
	}()

	<-time.After(1 * time.Second)
	zap.S().Info("Ready to receive requests, notify h@h server.")
	if err := h.HC.NotifyStarted(); err != nil {
		exit(errors.Wrap(err, "notify h@h p2p server when started"))
	}
	zap.S().Info("Finished notify h@h server, it's status should be 'Online' on your h@h panel.")

	<-ctx.Done()
	zap.S().Info("Graceful shutdown...")

	zap.S().Info("Notify h@h server, it won't accept new connections.")
	if err := h.HC.NotifyShutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "notify h@h p2p server when shutdown: %s", err)
	}
	zap.S().Info("Finished notify h@h server, successful shutdown.")

	zap.S().Info("Wait HTTP server to exit...")
	wg.Wait()

	zap.S().Info("Hath Exit.")
}

func exit(err error) {
	fmt.Fprintf(os.Stderr, "Server failed to start: %s", err)
	os.Exit(1)
}

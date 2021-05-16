package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/mayocream/hath-go/pkg/hath"
	hServer "github.com/mayocream/hath-go/server"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Server ...
type Server struct {
	hath *hServer.Hath
}

// NewServer ...
func NewServer(hath *hServer.Hath) *Server {
	return &Server{
		hath: hath,
	}
}

// Serve ...
func (s *Server) Serve(ctx context.Context) error {
	srv := fiber.New()
	srv.All("/h/*", s.hvFileHandler)
	srv.All("/servercmd/*", s.serverCmdHandler)
	srv.All("/t/*", s.testHandler)

	if viper.GetBool("debug") {
		zap.S().Info("Fiber server now record http request to logs")
		logConf := logger.ConfigDefault
		logConf.Output = os.Stdout
		srv.Use(logger.New(logConf))
	}

	tlsConfig, err := s.hath.TLSConfig()
	if err != nil {
		return err
	}

	zap.S().Info("HTTP Server will serve at: %v", s.hath.Addr())
	ln, err := net.Listen("tcp", fmt.Sprintf(":%v", s.hath.Addr()))
	if err != nil {
		return err
	}
	ln = tls.NewListener(ln, tlsConfig)
	zap.S().Info("HTTPS Server enabled.")

	go func() {
		<-ctx.Done()
		zap.S().Info("HTTP server graceful shutdown...")
		srv.Shutdown()
		zap.S().Info("Finished HTTP server graceful shutdown.")
	}()

	return srv.Listener(ln)
}

func (s *Server) hvFileHandler(c *fiber.Ctx) error {
	params := c.Params("*")
	split := strings.Split(params, "/")
	if len(split) != 3 {
		return &fiber.Error{
			Code: http.StatusBadRequest,
		}
	}

	hv, err := s.hath.HandleHV(split[0], split[1], split[2])
	if err != nil {
		return wrapErr(err)
	}

	c.Send(hv.Data)
	return nil
}

func (s *Server) serverCmdHandler(c *fiber.Ctx) error {
	params := c.Params("*")
	split := strings.Split(params, "/")
	if len(split) != 4 {
		return &fiber.Error{
			Code: http.StatusBadRequest,
		}
	}

	ip := c.Context().RemoteIP().String()
	result, err := s.hath.HandleHathCmd(ip, split[0], split[1], split[2], split[3])
	if err != nil {
		return wrapErr(err)
	}

	c.Send(result)
	return nil
}

func (s *Server) testHandler(c *fiber.Ctx) error {
	params := c.Params("*")
	split := strings.Split(params, "/")
	if len(split) != 3 {
		return &fiber.Error{
			Code: http.StatusBadRequest,
		}
	}

	result, err := s.hath.HandleTest(split[0], split[1], split[2])
	if err != nil {
		return err
	}

	c.Send(result)
	return nil
}

func wrapErr(err error) error {
	if err, ok := err.(*hath.HTTPErr); ok {
		return &fiber.Error{
			Code:    err.Status,
			Message: err.Error(),
		}
	}

	return err
}

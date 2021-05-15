package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"

	"github.com/gofiber/fiber/v2"
	"github.com/mayocream/hath-go/pkg/hath"
	hServer "github.com/mayocream/hath-go/server"
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
	srv.All("/h/:fileid/:additional/:filename", s.hvFileHandler)
	srv.All("/servercmd/:command/:additional/:time/:key", s.hvFileHandler)

	tlsConfig, err := s.hath.TLSConfig()
	if err != nil {
		return err
	}

	ln, _ := net.Listen("tcp", fmt.Sprintf(":%v", s.hath.Addr()))
	ln = tls.NewListener(ln, tlsConfig)

	go func() {
		<-ctx.Done()
		srv.Shutdown()
	}()

	return srv.Listener(ln)
}

func (s *Server) hvFileHandler(c *fiber.Ctx) error {
	hv, err := s.hath.HandleHV(c.Params("fileid"), c.Params("additional"), c.Params("filename"))
	if err != nil {
		return wrapErr(err)
	}

	c.Send(hv.Data)
	return nil
}

func (s *Server) serverCmdHandler(c *fiber.Ctx) error {
	ip := c.Context().RemoteAddr().String()
	result, err := s.hath.HandleHathCmd(ip, c.Params("command"), c.Params("additional"), c.Params("time"), c.Params("key"))
	if err != nil {
		return wrapErr(err)
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

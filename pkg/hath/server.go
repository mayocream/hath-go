package hath

import (
	"errors"
	"fmt"
	"net"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mayocream/hath-go/pkg/hath/util"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

// Server p2p server
type Server struct {
	hc     *Client
	logger *zap.SugaredLogger
}

// NewServer ...
func NewServer() (*Server, error) {
	return &Server{}, nil
}

// ParseRPCRequest only GET/HEAD methods avaliable on rpc call,
// 	we will receive request from h@h server,
//	params are included in HTTP path.
//	ps: the original JAVA server parse raw HTTP line protocol,
//	using regex to match HTTP method, manauly split first line,
//	just like "GET /u/18544?s=48&v=4 HTTP/2", it's not elegant.
func (s *Server) ParseRPCRequest(req *http.Request) (interface{}, error) {
	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		return nil, errors.New("invalid rpc call")
	}

	return nil, nil
}

// HandleHV ...
//	form: /h/$fileid/$additional/$filename
func (s *Server) HandleHV(w http.ResponseWriter, r *http.Request) {
}

// HandleHathCmd ...
//	form: /servercmd/$command/$additional/$time/$key
func (s *Server) HandleHathCmd(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	cmd := vars["cmd"]
	add := vars["add"]
	srvTime := cast.ToInt(vars["time"])
	key := vars["key"]

	// only allow API Server rquests
	ip := net.ParseIP(r.RemoteAddr)

	s.logger.With("params", vars, "ip", ip).Info("ServerCmd, received event.")

	// We might not check source ip for it alreay contains hash digital sign.
	// s.hc.RPCServers.RLock()
	// if _, ok := s.hc.RPCServers.Hosts[ip.String()]; !ok {
	// 	s.hc.RPCServers.RUnlock()
	// 	w.WriteHeader(http.StatusForbidden)
	// 	s.logger.With("params", vars, "ip", ip).Warn("ServerCmd, unknown IP.")
	// 	return
	// }
	// s.hc.RPCServers.RUnlock()

	exptKey := util.SHA1(fmt.Sprintf("hentai@home-servercmd-%s-%s-%s-%s-%s",
		cmd, add, cast.ToString(s.hc.ClientID), cast.ToString(srvTime), s.hc.ClientKey))
	if (srvTime-util.SystemTime()) > MaxKeyTimeDrift || exptKey != key {
		w.WriteHeader(http.StatusForbidden)
		s.logger.With("params", vars, "ip", ip).Warn("ServerCmd, invalid request.")
		return
	}

	if err := s.execAPICmd(w, cmd, add); err != nil {
		s.logger.With("params", vars, "ip", ip).Errorf("ServerCmd, exec: %s", err)
		return
	}
}

func (s *Server) execAPICmd(w http.ResponseWriter, cmd string, add string) error {
	// although we use iso-8859-1 encoding, generaly we don't use unicode strings,
	//	it's fine just print utf8 encoding.
	w.Header().Set("Content-Type:", "text/html; charset=ISO-8859-1")

	switch cmd {
		// health check
	case "still_alive":
		w.Write([]byte("I feel FANTASTIC and I'm still alive"))
	case "threaded_proxy_test":
	case "speed_test":
	case "refresh_settings":
	case "start_downloader":
	case "refresh_certs":
	default:
		w.Write([]byte("INVALID_COMMAND"))
		return errors.New("invalid command")
	}

	return nil
}
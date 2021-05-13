package hath

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"
	"github.com/mayocream/hath-go/pkg/hath/util"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

func init() {
	rand.Seed(time.Now().Unix())
}

// Server p2p server
type Server struct {
	hc     *Client
	dl     *Downloader
	logger *zap.SugaredLogger
	stor   *Storage
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
func (s *Server) HandleHV(fileID string, addStr string, fileName string) (*HVFile, error) {
	add := util.ParseAddition(addStr)
	hvFile, err := NewHVFileFromFileID(fileID)
	if err != nil {
		s.logger.With("fileID", fileID).Warnf("HVFile, %s", err)
		return nil, err
	}

	keystampRejected := true
	if keystamp, ok := add["keystamp"]; ok {
		parts := strings.Split(keystamp, "-")
		if len(parts) == 2 {
			keystampTime := cast.ToInt(parts[0])
			k := util.SHA1(fmt.Sprintf("%s-%s-%s-hotlinkthis", cast.ToString(keystampTime), fileID, s.hc.ClientKey))
			if math.Abs(float64(util.SystemTime()-keystampTime)) < 900 && strings.ToLower(parts[1]) == k[:10] {
				keystampRejected = false
			}
		}
	}

	fileIndex := cast.ToInt(add["fileindex"])
	xres := add["xres"]

	if keystampRejected {
		return nil, NewHTTPErr(http.StatusForbidden, errors.New("keystamp rejected"))
	}

	if fileIndex == 0 || xres == "" {
		return nil, NewHTTPErr(http.StatusNotFound, errors.New("Invalid or missing arguments"))
	}

	hv, err := s.stor.GetHVFile(hvFile)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, NewHTTPErr(http.StatusNotFound, ErrNotFound)
		}
		// TODO proxy download
	}
	return hv, nil
}

// HandleTest ...
// 	form: /t/$testsize/$testtime/$testkey
func (s *Server) HandleTest(testSize, testTime int, testKey string) {
	// TODO return random bytes
}

// HandleHathCmd ...
// TODO translate into general struct, to support Fiber web framework.
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
	addParams := util.ParseAddition(add)
	// although we use iso-8859-1 encoding, generaly we don't use unicode strings,
	//	it's fine just print utf8 encoding.
	w.Header().Set("Content-Type:", "text/html; charset=ISO-8859-1")

	switch cmd {
	// health check
	case "still_alive":
		w.Write([]byte("I feel FANTASTIC and I'm still alive"))
	case "threaded_proxy_test":
		result, err := s.execDownloadTest(addParams)
		if err != nil {
			return err
		}
		w.Write(result)
	case "speed_test":
		// TODO return random bytes
	case "refresh_settings":
		s.hc.FetchRemoteSettings(true)
	case "start_downloader":
		// ignore it, we will init Download at started.
	case "refresh_certs":
		tlsCert, err := s.hc.GetTLSCertificate()
		if err != nil {
			return err
		}
		s.hc.Certificate.StoreCertificate(tlsCert)
	default:
		w.Write([]byte("INVALID_COMMAND"))
		return errors.New("invalid command")
	}

	return nil
}

func (s *Server) execDownloadTest(add map[string]string) ([]byte, error) {
	host := add["hostname"] + ":" + add["port"]
	protocol := add["protocol"]
	// default scheme
	if protocol == "" {
		protocol = "http"
	}
	testSize := cast.ToInt(add["testsize"])
	testCount := cast.ToInt(add["testcount"])
	testTime := cast.ToInt(add["testtime"])
	testKey := add["testkey"]

	var totalTimeMs int64
	totalSuccess := int64(testCount)

	wg := new(sync.WaitGroup)
	for i := 0; i < testCount; i++ {
		fileURL := &url.URL{
			Scheme: protocol,
			Host:   host,
			Path:   fmt.Sprintf("/t/%v/%v/%s/%v", testSize, testTime, testKey, rand.Int()),
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			duration, err := s.dl.DiscardDownload(fileURL.String())
			if err != nil {
				atomic.AddInt64(&totalSuccess, -1)
				return
			}
			atomic.AddInt64(&totalTimeMs, duration.Milliseconds())
		}()
	}

	wg.Wait()

	result := fmt.Sprintf("OK:%v-%v", totalSuccess, totalTimeMs)

	return []byte(result), nil
}

package hath

import (
	"crypto/tls"
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

	"github.com/mayocream/hath-go/pkg/hath/util"
	"github.com/spf13/cast"
	"go.uber.org/zap"
)

func init() {
	rand.Seed(time.Now().Unix())
}

// Config ...
type Config struct {
	Settings    `mapstructure:",squash"`
	StorageConf `mapstructure:",squash"`
}

// Server p2p server
type Server struct {
	HC     *Client
	DL     *Downloader
	logger *zap.SugaredLogger
	Stor   *Storage
}

// NewServer ...
func NewServer(config Config) (*Server, error) {
	hc, err := NewClient(config.Settings)
	if err != nil {
		return nil, err
	}
	stor, err := NewStorage(config.StorageConf)
	if err != nil {
		return nil, err
	}
	dl := NewDownloader()
	logger := zap.S().Named("hath")
	return &Server{
		DL:     dl,
		HC:     hc,
		Stor:   stor,
		logger: logger,
	}, nil
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
			k := util.SHA1(fmt.Sprintf("%s-%s-%s-hotlinkthis", cast.ToString(keystampTime), fileID, s.HC.ClientKey))
			if math.Abs(float64(util.SystemTime()-keystampTime)) < 900 && strings.ToLower(parts[1]) == k[:10] {
				keystampRejected = false
			}
		}
	}

	fileIndex := cast.ToInt(add["fileindex"])
	xres := add["xres"]

	// 403 Forbidden
	if keystampRejected {
		return nil, NewHTTPErr(http.StatusForbidden, errors.New("keystamp rejected"))
	}

	// check params
	if fileIndex == 0 || xres == "" {
		return nil, NewHTTPErr(http.StatusNotFound, errors.New("Invalid or missing arguments"))
	}

	hv, err := s.Stor.GetHVFile(hvFile)
	if err != nil {
		// file not exsit on local disk
		var validStaticRange bool
		s.HC.RemoteSettings.RLock()
		_, validStaticRange = s.HC.RemoteSettings.StaticRanges[fileID]
		s.HC.RemoteSettings.RUnlock()
		if errors.Is(err, ErrNotFound) && validStaticRange {
			// download it then return
			urls, err := s.HC.GetStaticRangeFetchURL(cast.ToString(fileIndex), xres, fileID)
			if err != nil {
				s.logger.With("fileID", fileID).Errorf("Fetch static range url: %s", err)
				return nil, NewHTTPErr(http.StatusNotFound, err)
			}
			if len(urls) == 0 {
				return nil, NewHTTPErr(http.StatusNotFound, ErrNotFound)
			}
			// proxy download
			data, err := s.DL.MultipleSourcesDownload(urls, hvFile)
			if err != nil {
				s.logger.With("fileID", fileID).Errorf("Proxy download failed: %s", err)
				return nil, NewHTTPErr(http.StatusNotFound, err)
			}
			hvFile.Data = data
			return hvFile, nil
		}
		return nil, NewHTTPErr(http.StatusNotFound, ErrNotFound)
	}
	return hv, nil
}

// HandleTest ...
// 	form: /t/$testsize/$testtime/$testkey
func (s *Server) HandleTest(sizeStr, timeStr, key string) ([]byte, error) {
	size := cast.ToInt(sizeStr)
	// srvTime := cast.ToInt(timeStr)
	// TODO auth
	buf := make([]byte, size)
    _, err := rand.Read(buf)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// HandleHathCmd ...
// TODO translate into general struct, to support Fiber web framework.
//	form: /servercmd/$command/$additional/$time/$key
func (s *Server) HandleHathCmd(serverIP, cmd, add, serverTime, key string) ([]byte, error) {
	vars := fmt.Sprintf("ip: %s, cmd: %s, add: %s, time: %s, key: %s", serverIP, cmd, add, serverTime, key)

	srvTime := cast.ToInt(serverTime)
	// only allow API Server rquests
	ip := net.ParseIP(serverIP)

	s.logger.With("params", vars, "ip", ip).Info("ServerCmd, received event.")

	// We might not check source ip for it alreay contains hash digital sign.
	// s.HC.RPCServers.RLock()
	// if _, ok := s.HC.RPCServers.Hosts[ip.String()]; !ok {
	// 	s.HC.RPCServers.RUnlock()
	// 	w.WriteHeader(http.StatusForbidden)
	// 	s.logger.With("params", vars, "ip", ip).Warn("ServerCmd, unknown IP.")
	// 	return
	// }
	// s.HC.RPCServers.RUnlock()

	exptKey := util.SHA1(fmt.Sprintf("hentai@home-servercmd-%s-%s-%s-%s-%s",
		cmd, add, cast.ToString(s.HC.ClientID), cast.ToString(srvTime), s.HC.ClientKey))
	if (srvTime-util.SystemTime()) > MaxKeyTimeDrift || exptKey != key {
		s.logger.With("params", vars, "ip", ip).Warn("ServerCmd, invalid request.")
		return nil, NewHTTPErr(http.StatusForbidden, errors.New("invalid ke"))
	}

	result, err := s.execAPICmd(cmd, add)
	if err != nil {
		s.logger.With("params", vars, "ip", ip).Errorf("ServerCmd, exec: %s", err)
		return nil, NewHTTPErr(http.StatusBadRequest, err)
	}

	return result, nil
}

func (s *Server) execAPICmd(cmd string, add string) ([]byte, error) {
	addParams := util.ParseAddition(add)

	switch cmd {
	// health check
	case "still_alive":
		return []byte("I feel FANTASTIC and I'm still alive"), nil
	case "threaded_proxy_test":
		result, err := s.execDownloadTest(addParams)
		if err != nil {
			return nil, err
		}
		return result, nil
	case "speed_test":
		// return random bytes
		size := cast.ToInt(addParams["testsize"])
		if size == 0 {
			size = 1000000
		}
		buf := make([]byte, size)
		_, err := rand.Read(buf)
		if err != nil {
			return nil, err
		}
		return buf, nil
	case "refresh_settings":
		s.HC.FetchRemoteSettings(true)
	case "start_downloader":
		// ignore it, we will init Download at started.
	case "refresh_certs":
		tlsCert, err := s.HC.GetTLSCertificate()
		if err != nil {
			return nil, err
		}
		s.HC.Certificate.StoreCertificate(tlsCert)
	default:
		return []byte("INVALID_COMMAND"), errors.New("invalid command")
	}

	return nil, nil
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
			duration, err := s.DL.DiscardDownload(fileURL.String())
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

// TLSConfig ...
func (s *Server) TLSConfig() (*tls.Config, error) {
	s.logger.Info("init server tls config")
	cert, err := s.HC.GetTLSCertificate()
	if err != nil {
		return nil, err
	}
	s.HC.Certificate.StoreCertificate(cert)
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return s.HC.Certificate.GetCertificate()
		},
	}, nil
}

// Addr ...
func (s *Server) Addr() int {
	return s.HC.RemoteSettings.ServerPort
}

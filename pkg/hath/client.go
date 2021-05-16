// Package hath client/server side impl
package hath

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudflare/cfssl/helpers"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"golang.org/x/crypto/pkcs12"

	"github.com/mayocream/hath-go/pkg/hath/util"
	"github.com/mayocream/hath-go/pkg/wrr"
)

// Settings stands for client side config.
type Settings struct {
	ClientID  string `mapstructure:"client_id"`
	ClientKey string `mapstructure:"client_key"`
}

// RemoteSettings config from remote server, overwrite local config.
//	it can be modified from server side, so it can be hot reload.
type RemoteSettings struct {
	sync.RWMutex

	ServerPort   int `yaml:"server-port" mapstructure:"server-port"`
	StaticRanges map[string]int

	RawSettings map[string]string
}

// RPCServers multi-server for rpc call, using weighted round-robin aglo
//	to load balancing.
type RPCServers struct {
	sync.RWMutex

	Hosts    map[string]int
	Balancer wrr.WRR
}

// RPCResponse general hath response from server
type RPCResponse struct {
	Status  RPCStatus  `json:"status"`
	Payload RPCPayload `json:"payload"`
	Host    string     `json:"host"`
}

// Certificate tls cert for multi-goroutine
type Certificate struct {
	*tls.Certificate

	mu sync.RWMutex
}

// StoreCertificate store cert
func (c *Certificate) StoreCertificate(cert *tls.Certificate) {
	defer c.mu.Unlock()
	c.mu.Lock()

	c.Certificate = cert
}

// GetCertificate get cert
func (c *Certificate) GetCertificate() (*tls.Certificate, error) {
	defer c.mu.RUnlock()
	c.mu.RLock()
	if c.Certificate == nil {
		return nil, errors.New("cert not available")
	}

	return c.Certificate, nil
}

// RPCPayload rpc payload data.
//	There might be 2 forms of respopnse,
//	1. key and value pairs
//	2. line data
type RPCPayload []string

// KeyValues kvs for settings.
func (p RPCPayload) KeyValues() map[string]string {
	kv := make(map[string]string, len(p))
	for _, s := range p {
		split := strings.Split(s, "=")
		if len(split) == 2 {
			kv[strings.ReplaceAll(split[0], "-", "_")] = split[1]
		}
	}
	return kv
}

// URLs each line contains one url
func (p RPCPayload) URLs() []*url.URL {
	ul := make([]*url.URL, 0, len(p))
	for _, s := range p {
		u, _ := url.Parse(s)
		if u != nil {
			ul = append(ul, u)
		}
	}
	return ul
}

// Client connects to hath server.
type Client struct {
	Settings
	RemoteSettings RemoteSettings

	RPCServers RPCServers

	http *resty.Client

	serverTimeDelta int64
	Certificate     *Certificate
}

// NewClient creates new client.
func NewClient(config Settings) (*Client, error) {
	if config.ClientID == "" || config.ClientKey == "" {
		return nil, errors.New("id/key missing")
	}
	// client id must be integer more than 0
	if len(config.ClientID) == 0 {
		return nil, errors.New("invalid client id")
	}

	if ok, _ := regexp.MatchString(`^[a-zA-Z0-9]{`+strconv.Itoa(ClientKeyLength)+`}$`, config.ClientKey); !ok {
		return nil, errors.New("invalid client key")
	}
	c := &Client{
		Settings: config,
		http: resty.NewWithClient(&http.Client{
			Transport: http.DefaultTransport,
			Timeout:   60 * time.Second,
		}).SetHeader("Connection", "Close").
			SetHeader("User-Agent", "Hentai@Home "+ClientVersion).
			// SetRetryCount(3).
			EnableTrace().
			SetDebug(cast.ToBool(os.Getenv("HATH_HTTP_DEBUG"))),
		Certificate: new(Certificate),
	}
	// Init
	zap.S().Info("sync server time delta")
	c.SyncTimeDelta()
	zap.S().Info("fetch remote settings")
	c.FetchRemoteSettings(false) // not running
	return c, nil
}

var (
	// ErrRespIsNull server error
	ErrRespIsNull = errors.New("obb response")
	// ErrTemporarilyUnavailable another server error
	ErrTemporarilyUnavailable = errors.New("temporarily unavailable")
	// ErrConnectTestFailed The server failed to verify that this client is online and available from the Internet.
	ErrConnectTestFailed = errors.New("failed external connection test")
	// ErrIPAddressInUse The server detected that another client was already connected from this computer or local network.
	// 	You can only have one client running per public IP address.
	ErrIPAddressInUse = errors.New("another client was already connected")
	// ErrClientIDInUse The server detected that another client is already using this client ident.
	//	If you want to run more than one client, you have to apply for additional idents.
	ErrClientIDInUse = errors.New("another client is already using this client ident")
)

// RPCRawRequest will retry 3 times when failed, then downgrade rpc severs
//	TODO improve load balancer for less RTT
func (c *Client) RPCRawRequest(uri *url.URL) (*RPCResponse, error) {
	resp, err := c.http.R().Get(uri.String())
	if err != nil {
		return nil, err
	}
	ti := resp.Request.TraceInfo()
	log := zap.S().Named("Hath-Client").With("url", uri.String(),
		"totalTime", ti.TotalTime.String(),
		"connTime", ti.ConnTime.String(),
		"responseTime", (ti.ServerTime + ti.ResponseTime).String())
	if len(resp.Body()) == 0 {
		log.Warnf("HathRPC, http code: %v, empty body.", resp.StatusCode())
		return nil, ErrRespIsNull
	}

	split := strings.Split(string(resp.Body()), "\n")
	if len(split) < 1 {
		log.Warnf("HathRPC, http code: %v, missing rpc status.", resp.StatusCode())
		return nil, ErrRespIsNull
	}

	status := split[0]

	if resp.StatusCode() > 200 || status != "OK" {
		log.Warnf("HathRPC, http code: %v, status: %s", resp.StatusCode(), status)
	} else {
		log.Infof("HathRPC, http code: %v, status: %s", resp.StatusCode(), status)
	}

	if status == "OK" {
		// filter results
		payload := make([]string, 0, len(split)-1)
		// avoid out of range
		if len(split) > 1 {
			for _, s := range split[1:] {
				if v := strings.Trim(s, ""); v != "" {
					payload = append(payload, v)
				}
			}
		}
		return &RPCResponse{
			Status:  ResponseStatusOK,
			Payload: payload,
			Host:    uri.Host,
		}, nil
	}

	if status == "KEY_EXPIRED" {
		// refresh key and retry
		if err := c.SyncTimeDelta(); err != nil {
			return nil, errors.Wrap(err, "key expired, retry failed")
		}
		if _, err := c.FetchRemoteSettings(true); err != nil {
			return nil, errors.Wrap(err, "key expired, retry failed")
		}
		return c.RPCRawRequest(uri)
	}

	if strings.HasPrefix(status, "TEMPORARILY_UNAVAILABLE") {
		return nil, ErrTemporarilyUnavailable
	}

	if strings.HasPrefix(status, "FAIL_CONNECT_TEST") {
		return nil, ErrConnectTestFailed
	}

	if strings.HasPrefix(status, "FAIL_OTHER_CLIENT_CONNECTED") {
		return nil, ErrIPAddressInUse
	}

	if strings.HasPrefix(status, "FAIL_CID_IN_USE") {
		return nil, ErrIPAddressInUse
	}

	return nil, fmt.Errorf("unknown: %s", status)
}

// correctedTime server might need UTC time
func (c *Client) correctedTime() int {
	return util.SystemTime() + int(atomic.LoadInt64(&c.serverTimeDelta))
}

// GetRPCURL url query string holds params.
func (c *Client) GetRPCURL(act Action, add string) *url.URL {
	u := &url.URL{
		Scheme: ClientRPCProtocol,
		Path:   ClientRPCFile,
		Host:   c.GetRPCHost(),
	}
	if act == ActionServerStat {
		q := make(url.Values, 2)
		q.Add("clientbuild", strconv.Itoa(ClientBuild))
		q.Add("act", string(act))
		u.RawQuery = q.Encode()
		return u
	}

	u.RawQuery = c.getURLQuery(act, add).Encode()
	return u
}

// RPCRequest general rpc call.
func (c *Client) RPCRequest(act Action, add string) (*RPCResponse, error) {
	return c.RPCRawRequest(c.GetRPCURL(act, add))
}

func (c *Client) getURLQuery(act Action, add string) url.Values {
	correctedTime := c.correctedTime()
	actKey := util.SHA1(fmt.Sprintf("hentai@home-%s-%s-%s-%s-%s",
		string(act), add, c.ClientID, strconv.Itoa(correctedTime), c.ClientKey))

	q := make(url.Values, 6)
	q.Add("clientbuild", strconv.Itoa(ClientBuild))
	q.Add("act", string(act))
	q.Add("cid", c.ClientID)
	q.Add("acttime", strconv.Itoa(correctedTime))
	q.Add("actkey", actKey)
	return q
}

// GetRPCHost wrr load balancer
func (c *Client) GetRPCHost() string {
	defer c.RPCServers.RUnlock()
	c.RPCServers.RLock()

	if len(c.RPCServers.Hosts) > 0 {
		return c.RPCServers.Balancer.Next().(string)
	}

	return ClientRPCHost
}

// SyncTimeDelta sync clock with hath server
func (c *Client) SyncTimeDelta() error {
	resp, err := c.RPCRequest(ActionServerStat, "")
	if err != nil {
		return err
	}

	srvTimeStr, ok := resp.Payload.KeyValues()["server_time"]
	if !ok {
		return errors.New("cannot get server time")
	}

	srvTime, err := strconv.ParseInt(srvTimeStr, 10, 0)
	if err != nil {
		return errors.New("failed to parse server time")
	}

	delta := srvTime - time.Now().Unix()
	// avoid data race
	atomic.StoreInt64(&c.serverTimeDelta, delta)

	return nil
}

// FetchRemoteSettings fetch client settings from h@h, priority more than local config
func (c *Client) FetchRemoteSettings(isRunning bool) (*RPCResponse, error) {
	// action can be different from server side logic,
	//	though it returns same response.
	// RAW: this MUST NOT be called after the client has started up,
	//	as it will clear out and reset the client on the server,
	//	leaving the client in a limbo until restart
	act := ActionClientLogin
	if isRunning {
		act = ActionClientSettings
	}
	resp, err := c.RPCRequest(act, "")
	if err != nil {
		return nil, err
	}

	payloadKvs := resp.Payload.KeyValues()

	if srvListStr, ok := payloadKvs["rpc_server_ip"]; ok {
		srvList := strings.Split(srvListStr, ";")
		hosts := make(map[string]int, len(srvList))
		balancer := wrr.NewEDF()
		for _, srv := range srvList {
			if srvIP := net.ParseIP(srv); srvIP != nil {
				hosts[srvIP.String()] = 10
				balancer.Add(srvIP.String(), 10)
			}
		}
		defer c.RPCServers.Unlock()
		c.RPCServers.Lock()

		c.RPCServers.Hosts = hosts
		c.RPCServers.Balancer = balancer
	}

	defer c.RemoteSettings.Unlock()
	c.RemoteSettings.Lock()
	c.RemoteSettings.RawSettings = payloadKvs

	if port, ok := payloadKvs["port"]; ok {
		c.RemoteSettings.ServerPort = cast.ToInt(port)
	}

	if staticRanges, ok := payloadKvs["static_ranges"]; ok {
		for _, str := range strings.Split(staticRanges, ";") {
			if len(str) == 4 {
				c.RemoteSettings.StaticRanges[str] = 1
			}
		}
	}

	return resp, nil
}

// GetRawPKCS12 raw pkck12 file from hath server, including 2 certs and 1 priv key,
//	2 certs for workload cert and intermediate cert. We need adition setps to handle.
func (c *Client) GetRawPKCS12() ([]byte, error) {
	certURL := c.GetRPCURL(ActionGetCertificate, "")
	resp, err := c.http.R().Get(certURL.String())
	if err != nil {
		return nil, err
	}

	return resp.Body(), nil
}

// GetTLSCertificate the server returns pkcs12 package,
//	to server contents from HTTPS we need tls.Certificate
//	to provide digital encrypt and verification.
func (c *Client) GetTLSCertificate() (*tls.Certificate, error) {
	pk, err := c.GetRawPKCS12()
	if err != nil {
		return nil, err
	}

	// We should using pkcs12 topem method to remove unsupported tags
	// 	clientkey is used to decode.
	// ref: https://github.com/golang/go/issues/23499#issuecomment-367849407
	// It's a workaround for golang pkcs12 package is only for a single file
	// 	contains only one key and one certificate.
	pemBlocks, err := pkcs12.ToPEM(pk, c.ClientKey)
	if err != nil {
		return nil, errors.Wrap(err, "pkcs12 decode")
	}

	var leafCert, intermediateCert *x509.Certificate
	var privkey []byte

	for _, block := range pemBlocks {
		p := pem.EncodeToMemory(block)

		if block.Type == "CERTIFICATE" {
			cert, err := helpers.ParseCertificatePEM(p)
			if err != nil {
				return nil, errors.Wrap(err, "parse cert")
			}
			// TODO support multi intermediate certs
			if cert.IsCA {
				intermediateCert = cert
			} else {
				leafCert = cert
			}
		} else if block.Type == "PRIVATE KEY" {
			key, err := helpers.ParsePrivateKeyPEMWithPassword(p, []byte(c.ClientKey))
			if err != nil {
				return nil, errors.Wrap(err, "parse key")
			}
			priv, err := x509.MarshalPKCS8PrivateKey(key)
			if err != nil {
				return nil, errors.Wrap(err, "marshal key")
			}
			privkey = pem.EncodeToMemory(&pem.Block{
				Type:  "PRIVATE KEY",
				Bytes: priv,
			})
		}
	}

	tlsCert, err := tls.X509KeyPair(helpers.EncodeCertificatesPEM([]*x509.Certificate{leafCert, intermediateCert}), privkey)
	if err != nil {
		return nil, errors.Wrap(err, "tls cert")
	}

	return &tlsCert, nil
}

// GetStaticRangeFetchURL ...
func (c *Client) GetStaticRangeFetchURL(fileIndex, xres, fileID string) ([]string, error) {
	resp, err := c.RPCRequest(ActionStaticRangeFetch, fmt.Sprintf("%s;%s;%s", fileIndex, xres, fileID))
	if err != nil {
		return nil, err
	}

	vurls := resp.Payload.URLs()
	urls := make([]string, 0, len(vurls))
	for _, u := range vurls {
		urls = append(urls, u.String())
	}

	return urls, err
}

// NotifyStarted notify h@h server we are ready to receive requests
func (c *Client) NotifyStarted() error {
	_, err := c.RPCRequest(ActionClientStart, "")
	if err != nil {
		return err
	}

	return nil
}

// NotifyShutdown notify h@h server we are shutdown
func (c *Client) NotifyShutdown() error {
	_, err := c.RPCRequest(ActionClientStop, "")
	if err != nil {
		return err
	}

	return nil
}

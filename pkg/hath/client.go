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
	"golang.org/x/crypto/pkcs12"

	"github.com/mayocream/hath-go/pkg/hath/util"
	"github.com/mayocream/hath-go/pkg/wrr"
)

// Settings stands for client side config.
type Settings struct {
	ClientID  int    `yaml:"clien-id" mapstructure:"clien-id"`
	ClientKey string `yaml:"client-key" mapstructure:"client-key"`
}

// RemoteSettings config from remote server, overwrite local config.
//	it can be modified from server side, so it can be hot reload.
type RemoteSettings struct {
	sync.RWMutex

	ServerPort int `yaml:"server-port" mapstructure:"server-port"`
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
}

// NewClient creates new client.
func NewClient(config Settings) (*Client, error) {
	if config.ClientID == 0 || config.ClientKey == "" {
		return nil, errors.New("id/key missing")
	}
	// client id must be integer more than 0
	if config.ClientID <= 0 {
		return nil, errors.New("invalid client id")
	}

	if ok, _ := regexp.MatchString(`^[a-zA-Z0-9]{`+strconv.Itoa(ClientKeyLength)+`}$`, config.ClientKey); !ok {
		return nil, errors.New("invalid client key")
	}
	c := &Client{
		Settings: config,
		http: resty.NewWithClient(&http.Client{
			Transport: http.DefaultTransport,
			Timeout:   5 * time.Second,
		}).SetHeader("Connection", "Close").
			SetHeader("User-Agent", "Hentai@Home "+ClientVersion).
			SetRetryCount(3).
			SetDebug(cast.ToBool(os.Getenv("HATH_HTTP_DEBUG"))),
	}
	// Init
	c.SyncTimeDelta()
	c.FetchRemoteSettings(false) // not running
	return c, nil
}

var (
	// ErrRespIsNull server error
	ErrRespIsNull = errors.New("obb response")
	// ErrTemporarilyUnavailable another server error
	ErrTemporarilyUnavailable = errors.New("temporarily unavailable")
)

// RPCRawRequest will retry 3 times when failed, then downgrade rpc severs
//	TODO improve load balancer for less RTT
func (c *Client) RPCRawRequest(uri *url.URL) (*RPCResponse, error) {
	resp, err := c.http.R().Get(uri.String())
	if err != nil {
		return nil, err
	}
	if len(resp.Body()) == 0 {
		return nil, ErrRespIsNull
	}

	split := strings.Split(string(resp.Body()), "\n")
	if len(split) < 1 {
		return nil, ErrRespIsNull
	}

	status := split[0]
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
		string(act), add, strconv.Itoa(c.ClientID), strconv.Itoa(correctedTime), c.ClientKey))

	q := make(url.Values, 6)
	q.Add("clientbuild", strconv.Itoa(ClientBuild))
	q.Add("act", string(act))
	q.Add("cid", strconv.Itoa(c.ClientID))
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
	act := ActionClientLogin
	if isRunning {
		act = ActionClientSettings
	}
	resp, err := c.RPCRequest(act, "")
	if err != nil {
		return nil, err
	}

	if srvListStr, ok := resp.Payload.KeyValues()["rpc_server_ip"]; ok {
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

// GetCertificate the server returns pkcs12 package,
//	to server contents from HTTPS we need tls.Certificate
//	to provide digital encrypt and verification.
func (c *Client) GetCertificate() (*tls.Certificate, error) {
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

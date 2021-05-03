package hath

import (
	"crypto/tls"
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
	"github.com/davecgh/go-spew/spew"
	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"golang.org/x/crypto/pkcs12"

	"github.com/mayocream/hath-go/pkg/hath/util"
	"github.com/mayocream/hath-go/pkg/wrr"
)

// Settings ...
type Settings struct {
	ClientID  int    `yaml:"clien-id" mapstructure:"clien-id"`
	ClientKey string `yaml:"client-key" mapstructure:"client-key"`

	RemoteSettings
}

// RemoteSettings ...
type RemoteSettings struct {
	ServerPort int `yaml:"server-port" mapstructure:"server-port"`
}

// RPCServers ...
type RPCServers struct {
	sync.RWMutex

	Hosts    []string
	Balancer wrr.WRR
}

// RPCResponse ...
type RPCResponse struct {
	Status  RPCStatus  `json:"status"`
	Payload RPCPayload `json:"payload"`
	Host    string     `json:"host"`
}

// RPCPayload payload data
type RPCPayload []string

// KeyValues kv for settings
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

// URLs ...
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

// Client connect to hath server
type Client struct {
	Settings

	RPCServers RPCServers

	http *resty.Client

	serverTimeDelta int64
}

// NewClient ...
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
	c.FetchRemoteSettings()
	return c, nil
}

var (
	ErrRespIsNull             = errors.New("obb response")
	ErrTemporarilyUnavailable = errors.New("temporarily unavailable")
)

// RPCRawRequest will retry 3 times when failed, then downgrade rpc severs
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
		if _, err := c.FetchRemoteSettings(); err != nil {
			return nil, errors.Wrap(err, "key expired, retry failed")
		}
		return c.RPCRawRequest(uri)
	}

	if strings.HasPrefix(status, "TEMPORARILY_UNAVAILABLE") {
		return nil, ErrTemporarilyUnavailable
	}

	return nil, fmt.Errorf("unknown: %s", status)
}

func (c *Client) correctedTime() int {
	return util.SystemTime() + int(atomic.LoadInt64(&c.serverTimeDelta))
}

// GetRPCURL ...
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

// RPCRequest ...
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

// GetRPCHost ...
func (c *Client) GetRPCHost() string {
	defer c.RPCServers.RUnlock()
	c.RPCServers.RLock()

	if len(c.RPCServers.Hosts) > 0 {
		return c.RPCServers.Balancer.Next().(string)
	}

	return ClientRPCHost
}

// SyncTimeDelta ...
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

// FetchRemoteSettings fetch settings from h@h
func (c *Client) FetchRemoteSettings() (*RPCResponse, error) {
	resp, err := c.RPCRequest(ActionClientLogin, "")
	if err != nil {
		return nil, err
	}

	if srvListStr, ok := resp.Payload.KeyValues()["rpc_server_ip"]; ok {
		srvList := strings.Split(srvListStr, ";")
		hosts := make([]string, 0, len(srvList))
		balancer := wrr.NewEDF()
		for _, srv := range srvList {
			if srvIP := net.ParseIP(srv); srvIP != nil {
				hosts = append(hosts, srvIP.String())
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

// GetRawPKCS12 ...
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

	// we should using pkcs12 topem method to remove unsupported tags
	// clientkey is used to decode
	pemBlocks, err := pkcs12.ToPEM(pk, c.ClientKey)
	if err != nil {
		return nil, errors.Wrap(err, "pkcs12 decode")
	}

	for _, block := range pemBlocks {
		p := pem.EncodeToMemory(block)

		if block.Type == "CERTIFICATE" {
			if _, err := helpers.ParseCertificatePEM(p); err != nil {
				return nil, errors.Wrap(err, "parse cert")
			}
		} else if block.Type == "PRIVATE KEY" {
			if _, err := helpers.ParsePrivateKeyPEMWithPassword(p, []byte(c.ClientKey)); err != nil {
				return nil, errors.Wrap(err, "parse key")
			}
		}
	}

	return nil, nil
}

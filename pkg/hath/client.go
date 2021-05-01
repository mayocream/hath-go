package hath

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/pkg/errors"

	"github.com/mayocream/hath-go/pkg/hath/util"
)

// Settings ...
type Settings struct {
	ClientID  string `yaml mapstructure:"clien-id"`
	ClientKey string `yaml mapstructure:"client-key"`
}

// RPCServers ...
type RPCServers struct {
	sync.RWMutex

	Hosts map[string]int
}

// RPCResponse ...
type RPCResponse struct {
	Raw []byte `json:"-"`

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
			kv[split[0]] = split[1]
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
}

// NewClient ...
func NewClient(config Settings) (*Client, error) {
	if config.ClientID == "" || config.ClientKey == "" {
		return nil, errors.New("id/key missing")
	}
	c := &Client{
		Settings: config,
		http: resty.NewWithClient(&http.Client{
			Transport: http.DefaultTransport,
			Timeout:   5 * time.Second,
		}).SetRetryCount(3),
	}
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
			Raw:     resp.Body(),
			Status:  ResponseStatusOK,
			Payload: payload,
			Host:    uri.Host,
		}, nil
	}

	if status == "KEY_EXPIRED" {
		// TODO refresh key and retry
		return c.RPCRawRequest(uri)
	}

	// TODO balancer down weight when failed
	if strings.HasPrefix(status, "TEMPORARILY_UNAVAILABLE") {
		return nil, ErrTemporarilyUnavailable
	}

	return nil, fmt.Errorf("unknown: %s", status)
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

func (c *Client) getURLQuery(act Action, add string) url.Values {
	sysTime := util.SystemTime()
	actKey := util.SHA1(fmt.Sprintf("hentai@home-%s-%s-%s-%s-%s",
		string(act), add, c.ClientID, strconv.Itoa(sysTime), c.ClientKey))

	q := make(url.Values, 6)
	q.Add("clientbuild", strconv.Itoa(ClientBuild))
	q.Add("act", string(act))
	q.Add("cid", c.ClientID)
	q.Add("acttime", strconv.Itoa(sysTime))
	q.Add("actkey", actKey)
	return q
}

// GetRPCHost ...
func (c *Client) GetRPCHost() string {
	defer c.RPCServers.RLock()

	c.RPCServers.RLock()
	if len(c.RPCServers.Hosts) > 0 {
		// TODO round-robin balancer
		return ""
	}

	return ClientRPCHost
}

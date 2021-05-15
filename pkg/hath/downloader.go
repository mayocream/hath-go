package hath

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
)

var copyBufPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, 4096)
	},
}

// Downloader ...
type Downloader struct {
	c *http.Client
}

func NewDownloader() *Downloader {
	return &Downloader{
		c: &http.Client{
			Transport: &http.Transport{
				IdleConnTimeout: 1 * time.Second,
				ResponseHeaderTimeout: 1 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 5 * time.Second,
		},
	}
}

// DiscardDownload ...
func (d *Downloader) DiscardDownload(uri string) (time.Duration, error) {
	startTime := time.Now()
	resp, err := d.c.Get(uri)
	if err != nil {
		return -1, errors.New("network error")
	}
	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return -1, errors.Wrap(err, "copy")
	}
	elapseTime := time.Since(startTime)
	return elapseTime, nil
}

// MultipleSourcesDownload download from multi sources
func (d *Downloader) MultipleSourcesDownload(sources []string, hv *HVFile) ([]byte, error) {
	vbuf := copyBufPool.Get()
	buf := vbuf.([]byte)
	defer copyBufPool.Put(vbuf)

	// TODO load balancer
	for _, s := range sources {
		resp, err := d.c.Get(s)
		if err != nil {
			continue
		}
		if resp.ContentLength < 0 || resp.ContentLength != int64(hv.Size) {
			continue
		}
	
		data := &bytes.Buffer{}
	
		if _, err := io.CopyBuffer(data, resp.Body, buf); err != nil {
			return nil, err
		}
		
		// TODO return io.reader
		return data.Bytes(), nil
	}

	return nil, errors.New("not avaliable sources")
}

// DummyDownload ...
func (d *Downloader) DummyDownload(uri string) ([]byte, error) {
	resp, err := d.c.Get(uri)
	if err != nil {
		return nil, errors.New("network error")
	}

	vbuf := copyBufPool.Get()
	buf := vbuf.([]byte)
	defer copyBufPool.Put(vbuf)

	data := &bytes.Buffer{}

	if _, err := io.CopyBuffer(data, resp.Body, buf); err != nil {
		return nil, err
	}

	return data.Bytes(), nil
}


// ProxyDownload ...
// TODO
func (d *Downloader) ProxyDownload(uri string) (io.Reader, error) {
	resp, err := d.c.Get(uri)
	if err != nil {
		return nil, errors.New("network error")
	}

	// body will be closed when server finished copy to output
	bodyReader := resp.Body

	return bodyReader, nil
}


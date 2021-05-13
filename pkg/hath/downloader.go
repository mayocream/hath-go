package hath

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"time"

	"github.com/pkg/errors"
)

// Downloader ...
type Downloader struct {
	c *http.Client
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

// ProxyDownload ...
func (d *Downloader) ProxyDownload(uri string, dst io.Reader) ([]byte, error) {
	resp, err := d.c.Get(uri)
	if err != nil {
		return nil, errors.New("network error")
	}
	defer resp.Body.Close()
	buf := make([]byte, 4096)
	buffer := bytes.NewBuffer(buf)
	readWriter := bufio.NewReadWriter(bufio.NewReader(resp.Body), bufio.NewWriter(buffer))
	

	return nil, nil
}




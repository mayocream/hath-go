package hath

import (
	"bufio"
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
	reader := bufio.NewReader(resp.Body)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return -1, errors.Wrap(err, "copy")
	}
	elapseTime := time.Since(startTime)
	return elapseTime, nil
}


package hath

import (
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestDownloader_DiscardDownload(t *testing.T) {
	d := &Downloader{
		c: http.DefaultClient,
	}

	cases := []struct {
		uri string
	}{
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter010/00-a.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/00-f.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/41.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/42.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/43.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/44.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/45.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/46.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/46.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/47.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/48.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/49.jpg"},
	}
	for _, cs := range cases {
		duration, err := d.DiscardDownload(cs.uri)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Println("duration: ", duration)
	}
}

func TestDownloader_DummyDownload(t *testing.T) {
	d := &Downloader{
		c: http.DefaultClient,
	}

	cases := []struct {
		uri string
	}{
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter010/00-a.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/00-f.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/41.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/42.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/43.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/44.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/45.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/46.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/46.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/47.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/48.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/49.jpg"},
	}

	for _, cs := range cases {
		data, err := d.DummyDownload(cs.uri)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("content length: %v", len(data))
	}
}

func TestDownloader_ProxyDownload(t *testing.T) {
	d := &Downloader{
		c: http.DefaultClient,
	}

	cases := []struct {
		uri string
	}{
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter010/00-a.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/00-f.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/41.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/42.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/43.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/44.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/45.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/46.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/46.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/47.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/48.jpg"},
		{uri: "https://raw.githubusercontent.com/lyy289065406/re0-web/master/gitbook/res/img/article/chapter030/49.jpg"},
	}

	for _, cs := range cases {
		reader, err := d.ProxyDownload(cs.uri)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("content length: %v", len(data))
	}
}

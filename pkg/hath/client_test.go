package hath

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/spf13/afero"
	"github.com/spf13/cast"
)

func testClient() *Client {
	if err := godotenv.Load("../../.env"); err != nil {
		panic(err)
	}
	c, err := NewClient(Settings{
		ClientID:  cast.ToInt(os.Getenv("HATH_CLIENT_ID")),
		ClientKey: os.Getenv("HATH_CLIENT_KEY"),
	})
	if err != nil {
		panic(err)
	}
	return c
}

func TestClient_GetRPCURL(t *testing.T) {
	c, err := NewClient(Settings{
		ClientID:  1,
		ClientKey: "12345678901234567890",
	})
	if err != nil {
		t.Fatal(err)
	}

	uri := c.GetRPCURL(ActionServerStat, "")
	t.Log("uri: ", uri)
}

func TestClient_RPCRawRequest(t *testing.T) {
	c, err := NewClient(Settings{
		ClientID:  1,
		ClientKey: "12345678901234567890",
	})
	if err != nil {
		t.Fatal(err)
	}

	u := c.GetRPCURL(ActionServerStat, "")
	resp, err := c.RPCRawRequest(u)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", resp)
}

func TestClient_FetchRemoteSettings(t *testing.T) {
	c := testClient()

	resp, err := c.FetchRemoteSettings(false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", resp)
}

func TestClient_GetRawPKCS12(t *testing.T) {
	c := testClient()

	resp, err := c.GetRawPKCS12()
	if err != nil {
		t.Fatal(err)
	}

	afero.WriteFile(afero.NewBasePathFs(afero.NewOsFs(), "."), "hathcert.p12", resp, 7777)
}

func TestClient_GetCertificate(t *testing.T) {
	c := testClient()

	_, err := c.GetCertificate()
	if err != nil {
		panic(err)
	}
}

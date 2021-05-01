package hath

import (
	"testing"
)

func TestClient_GetRPCURL(t *testing.T) {
	c, err := NewClient(Settings{
		ClientID:  "test",
		ClientKey: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	uri := c.GetRPCURL(ActionServerStat, "")
	t.Log("uri: ", uri)
}

func TestClient_RPCRawRequest(t *testing.T) {
	c, err := NewClient(Settings{
		ClientID:  "test",
		ClientKey: "test",
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

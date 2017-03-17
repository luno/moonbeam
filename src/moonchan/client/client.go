package client

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"moonchan/models"
)

var debugRPC = flag.Bool("debug_rpc", true, "Debug RPC")

type Client struct {
	host string
	c    http.Client
}

func NewClient(host string) *Client {
	return &Client{host: host}
}

func (c *Client) post(path string, req, resp interface{}) error {
	buf, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if *debugRPC {
		log.Printf("moonchan/client: Request to %s\n%s\n", path, string(buf))
	}

	hreq, err := http.NewRequest("POST", c.host+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}

	hresp, err := c.c.Do(hreq)
	if err != nil {
		return err
	}
	defer hresp.Body.Close()

	respBuf, err := ioutil.ReadAll(hresp.Body)
	if err != nil {
		return err
	}

	if *debugRPC {
		log.Printf("moonchan/client: Response from %s: %s\n%s\n",
			path, hresp.Status, string(respBuf))
	}

	if hresp.StatusCode != http.StatusOK {
		return fmt.Errorf("moonchan/client: http error code %d", hresp.StatusCode)
	}

	return json.Unmarshal(respBuf, resp)
}

func (c *Client) Create(req models.CreateRequest) (*models.CreateResponse, error) {
	var resp models.CreateResponse
	if err := c.post("/api/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Open(req models.OpenRequest) (*models.OpenResponse, error) {
	var resp models.OpenResponse
	if err := c.post("/api/open", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Send(req models.SendRequest) (*models.SendResponse, error) {
	var resp models.SendResponse
	if err := c.post("/api/send", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	var resp models.CloseResponse
	if err := c.post("/api/close", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

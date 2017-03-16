package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"moonchan/models"
)

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

	hreq, err := http.NewRequest("POST", c.host+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}

	hresp, err := c.c.Do(hreq)
	if err != nil {
		return err
	}
	defer hresp.Body.Close()

	if hresp.StatusCode != http.StatusOK {
		return fmt.Errorf("moonchan/client: http error code %d", hresp.StatusCode)
	}

	return json.NewDecoder(hresp.Body).Decode(resp)
}

func (c *Client) Create(req models.CreateRequest) (*models.CreateResponse, error) {
	var resp models.CreateResponse
	if err := c.post("/api/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

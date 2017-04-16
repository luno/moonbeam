package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"bitbucket.org/bitx/moonchan/models"
)

var debugRPC = flag.Bool("debug_rpc", true, "Debug RPC")

type Client struct {
	endpoint string
	c        *http.Client
}

func NewClient(c *http.Client, endpoint string) (*Client, error) {
	if strings.HasSuffix(endpoint, "/") {
		return nil, errors.New("endpoint must not have a trailing slash")
	}

	return &Client{
		endpoint: endpoint,
		c:        c,
	}, nil
}

func (c *Client) do(method, path string, req, resp interface{}) error {
	url := c.endpoint + path

	buf, err := json.Marshal(req)
	if err != nil {
		return err
	}

	if *debugRPC {
		log.Printf("moonchan/client: %s %s\n%s\n", method, url, string(buf))
	}

	hreq, err := http.NewRequest(method, url, bytes.NewReader(buf))
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
		log.Printf("moonchan/client: Response from %s %s: %s\n%s\n",
			method, url, hresp.Status, string(respBuf))
	}

	if hresp.StatusCode != http.StatusOK {
		return fmt.Errorf("moonchan/client: http error code %d", hresp.StatusCode)
	}

	return json.Unmarshal(respBuf, resp)
}

func (c *Client) Create(req models.CreateRequest) (*models.CreateResponse, error) {
	var resp models.CreateResponse
	if err := c.do(http.MethodPost, "/create", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func getChannelID(txid string, vout uint32) string {
	return fmt.Sprintf("%s-%d", txid, vout)
}

func (c *Client) Open(req models.OpenRequest) (*models.OpenResponse, error) {
	path := "/open/" + getChannelID(req.TxID, req.Vout)
	var resp models.OpenResponse
	if err := c.do(http.MethodPut, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Validate(req models.ValidateRequest) (*models.ValidateResponse, error) {
	path := "/validate/" + getChannelID(req.TxID, req.Vout)
	var resp models.ValidateResponse
	if err := c.do(http.MethodPut, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Send(req models.SendRequest) (*models.SendResponse, error) {
	path := "/send/" + getChannelID(req.TxID, req.Vout)
	var resp models.SendResponse
	if err := c.do(http.MethodPost, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Close(req models.CloseRequest) (*models.CloseResponse, error) {
	path := "/close/" + getChannelID(req.TxID, req.Vout)
	var resp models.CloseResponse
	if err := c.do(http.MethodDelete, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func (c *Client) Status(req models.StatusRequest) (*models.StatusResponse, error) {
	path := "/status/" + getChannelID(req.TxID, req.Vout)
	var resp models.StatusResponse
	if err := c.do(http.MethodGet, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

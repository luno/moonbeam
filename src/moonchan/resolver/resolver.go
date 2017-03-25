package resolver

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type DomainReceiver struct {
	URL string `json:"url"`
}

type Domain struct {
	Receivers []DomainReceiver `json:"receivers"`
}

type Resolver struct {
	Client      *http.Client
	DefaultPort int
}

func NewResolver() *Resolver {
	var c http.Client
	return &Resolver{
		Client: &c,
	}
}

func (r *Resolver) Resolve(domain string) (*url.URL, error) {
	if u, err := url.Parse(domain); err == nil {
		if u.Scheme != "" {
			return u, nil
		}
	}

	var rurl url.URL
	rurl.Scheme = "https"
	rurl.Host = domain
	rurl.Path = "/moonchan.json"

	if r.DefaultPort != 0 {
		rurl.Host += ":" + strconv.Itoa(r.DefaultPort)
	}

	resp, err := r.Client.Get(rurl.String())
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("bad http status code")
	}

	var d Domain
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}

	if len(d.Receivers) == 0 {
		return nil, errors.New("no url found")
	}

	return url.Parse(d.Receivers[0].URL)
}

var errInvalidAddress = errors.New("invalid address")

func ParseAddress(addr string) (string, string, error) {
	addr = strings.ToLower(addr)
	i := strings.Index(addr, "@")
	if i <= 0 {
		return "", "", errInvalidAddress
	}

	username := addr[:i]
	domain := addr[i+1:]

	if username == "" {
		return "", "", errInvalidAddress
	}
	if domain == "" {
		return "", "", errInvalidAddress
	}

	return username, domain, nil
}

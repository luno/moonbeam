package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"

	"bitbucket.org/bitx/moonchan/channels"
)

type Channel struct {
	Domain       string
	Host         string
	KeyPath      int
	ReceiverData []byte
	AuthToken    string

	PendingPayment []byte

	State channels.SharedState

	Payments [][]byte
}

type State struct {
	Seed           []byte
	XPrivKey       string
	KeyPathCounter int
	Channels       map[string]Channel
}

func (s *State) NextKey() int {
	c := s.KeyPathCounter
	s.KeyPathCounter++
	return c
}

func newState(net *chaincfg.Params) (*State, error) {
	seed, err := hdkeychain.GenerateSeed(hdkeychain.RecommendedSeedLen)
	if err != nil {
		return nil, err
	}
	key, err := hdkeychain.NewMaster(seed, net)
	if err != nil {
		return nil, err
	}

	return &State{
		Seed:     seed,
		XPrivKey: key.String(),
		Channels: make(map[string]Channel),
	}, nil
}

func getFilename(net *chaincfg.Params) string {
	return fmt.Sprintf("mbclient-state.%s.json", net.Name)
}

func save(net *chaincfg.Params, s *State) error {
	suffix := strconv.FormatInt(time.Now().Unix(), 10)

	name := getFilename(net)
	tmpName := name + ".tmp." + suffix

	f, err := os.Create(tmpName)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(s); err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, name)
}

func createNew(net *chaincfg.Params) (*State, error) {
	s, err := newState(net)
	if err != nil {
		return nil, err
	}
	if err := save(net, s); err != nil {
		return nil, err
	}
	return s, nil
}

func load(net *chaincfg.Params) (*State, error) {
	name := getFilename(net)
	f, err := os.Open(name)
	if os.IsNotExist(err) {
		return createNew(net)
	} else if err != nil {
		return nil, err
	}
	defer f.Close()

	var s State
	if err := json.NewDecoder(f).Decode(&s); err != nil {
		return nil, err
	}

	return &s, nil
}

var globalState *State

func getConfig() channels.SenderConfig {
	c := channels.DefaultSenderConfig
	c.Net = getNet().Name
	return c
}

func getChannel(id string) (*Channel, *channels.Sender, error) {
	s, ok := globalState.Channels[id]
	if !ok {
		return nil, nil, errors.New("unknown id")
	}

	privkey, _, err := loadkey(globalState, s.KeyPath)
	if err != nil {
		return nil, nil, err
	}

	sender, err := channels.LoadSender(getConfig(), s.State, privkey)
	if err != nil {
		return nil, nil, err
	}

	return &s, sender, nil
}

func storeChannel(id string, state channels.SharedState) error {
	c, ok := globalState.Channels[id]
	if !ok {
		return errors.New("channel does not exist")
	}
	c.State = state
	globalState.Channels[id] = c
	return nil
}

func storePendingPayment(id string, state channels.SharedState, p []byte) error {
	c, ok := globalState.Channels[id]
	if !ok {
		return errors.New("channel does not exist")
	}
	c.State = state
	if p == nil {
		c.Payments = append(c.Payments, c.PendingPayment)
		c.PendingPayment = nil
	} else {
		c.PendingPayment = p
	}
	globalState.Channels[id] = c
	return nil
}

func storeAuthToken(id string, authToken string) error {
	c, ok := globalState.Channels[id]
	if !ok {
		return errors.New("channel does not exist")
	}
	c.AuthToken = authToken
	globalState.Channels[id] = c
	return nil
}

func findForDomain(domain string) []string {
	var ids []string
	for id, c := range globalState.Channels {
		if c.Domain == domain && c.State.Status == channels.StatusOpen {
			ids = append(ids, id)
		}
	}
	return ids
}

package main

import (
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"

	"moonchan/channels"
	"moonchan/client"
	"moonchan/models"
	"moonchan/resolver"
)

func output(req interface{}, resp interface{}, err error) error {
	if err != nil {
		buf, _ := json.MarshalIndent(req, "", "    ")
		fmt.Printf("%s\n", string(buf))
	} else {
		buf, _ := json.MarshalIndent(resp, "", "    ")
		fmt.Printf("%s\n", string(buf))
	}
	return err
}

var testnet = flag.Bool("testnet", true, "Use testnet")

func getNet() *chaincfg.Params {
	if *testnet {
		return &chaincfg.TestNet3Params
	}
	return &chaincfg.MainNetParams
}

func loadkey(s *State, n int) (*btcec.PrivateKey, *btcutil.AddressPubKey, error) {
	net := getNet()

	ek, err := hdkeychain.NewKeyFromString(s.XPrivKey)
	if err != nil {
		return nil, nil, err
	}
	if !ek.IsForNet(net) {
		return nil, nil, errors.New("wrong network")
	}

	ek, err = ek.Child(uint32(n))
	if err != nil {
		return nil, nil, err
	}

	privKey, err := ek.ECPrivKey()
	if err != nil {
		return nil, nil, err
	}

	pk := (*btcec.PublicKey)(&privKey.PublicKey)
	pubkey, err := btcutil.NewAddressPubKey(pk.SerializeCompressed(), net)
	if err != nil {
		return nil, nil, err
	}

	return privKey, pubkey, nil
}

func getHttpClient() *http.Client {
	if *testnet {
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
		return &http.Client{Transport: tr}
	} else {
		return http.DefaultClient
	}
}

func getClient(id string) *client.Client {
	host := globalState.Channels[id].Host
	c := getHttpClient()
	return client.NewClient(c, host)
}

func getResolver() *resolver.Resolver {
	r := resolver.NewResolver()
	r.Client = getHttpClient()

	if *testnet {
		r.DefaultPort = 3211
	}

	return r
}

func create(args []string) error {
	domain := args[0]
	outputAddr := args[1]

	r := getResolver()
	hostURL, err := r.Resolve(domain)
	if err != nil {
		return err
	}
	host := hostURL.String()

	n := globalState.NextKey()
	privkey, _, err := loadkey(globalState, n)
	if err != nil {
		return err
	}

	net := getNet()
	s, err := channels.OpenChannel(net, privkey, outputAddr)
	if err != nil {
		return err
	}

	httpClient := getHttpClient()
	c := client.NewClient(httpClient, host)
	var req models.CreateRequest
	req.SenderPubKey = s.State.SenderPubKey.PubKey().SerializeCompressed()
	req.SenderOutput = s.State.SenderOutput
	resp, err := c.Create(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	receiverPubKey, err := btcutil.NewAddressPubKey(resp.ReceiverPubKey, net)
	if err != nil {
		return err
	}

	s.ReceivedPubKey(receiverPubKey, resp.ReceiverOutput)

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		return err
	}

	fmt.Printf("Funding address: %s\n", addr)

	// Sanity check to make sure client and server both agree on the state.
	if addr != resp.FundingAddress {
		return errors.New("state discrepancy")
	}

	if _, ok := globalState.Channels[resp.ID]; ok {
		return errors.New("reused channel id")
	}
	ss, err := s.State.ToSimple()
	if err != nil {
		return err
	}

	id := strconv.Itoa(n)
	globalState.Channels[id] = Channel{
		Domain:   domain,
		Host:     host,
		State:    *ss,
		KeyPath:  n,
		RemoteID: resp.ID,
	}

	fmt.Printf("%s\n", id)

	return nil
}

func fund(args []string) error {
	id := args[0]
	txid := args[1]
	vout, err := strconv.Atoi(args[2])
	if err != nil {
		return errors.New("invalid vout")
	}
	amount, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		return errors.New("invalid amount")
	}
	height := 0

	ch, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	sig, err := sender.FundingTxMined(txid, uint32(vout), amount, height)
	if err != nil {
		return err
	}

	c := getClient(id)
	req := models.OpenRequest{
		ID:        ch.RemoteID,
		TxID:      txid,
		Vout:      uint32(vout),
		SenderSig: sig,
	}
	resp, err := c.Open(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	return storeChannel(id, sender.State)
}

func send(args []string) error {
	target := args[0]
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return errors.New("invalid amount")
	}
	id := ""
	if len(args) > 2 {
		id = args[2]
	}

	if id == "" {
		_, domain, err := resolver.ParseAddress(target)
		if err != nil {
			return err
		}
		ids := findForDomain(domain)
		if len(ids) == 0 {
			return errors.New("no open channels to domain")
		}
		id = ids[0]
	}

	ch, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	sig, err := sender.PrepareSend(amount)
	if err != nil {
		return err
	}

	c := getClient(id)
	req := models.SendRequest{
		ID:        ch.RemoteID,
		Amount:    amount,
		SenderSig: sig,
	}
	resp, err := c.Send(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	if err := sender.SendAccepted(amount); err != nil {
		return err
	}

	return storeChannel(id, sender.State)
}

func closeAction(args []string) error {
	id := args[0]

	ch, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	c := getClient(id)
	req := models.CloseRequest{
		ID: ch.RemoteID,
	}
	resp, err := c.Close(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", hex.EncodeToString(resp.CloseTx))

	return storeChannel(id, sender.State)
}

func refund(args []string) error {
	id := args[0]

	_, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	if sender.State.Status != channels.StatusOpen {
		txid := args[1]
		vout, err := strconv.Atoi(args[2])
		if err != nil {
			return errors.New("invalid vout")
		}
		amount, err := strconv.ParseInt(args[3], 10, 64)
		if err != nil {
			return errors.New("invalid amount")
		}
		_, err = sender.FundingTxMined(txid, uint32(vout), amount, 0)
		if err != nil {
			return err
		}
		if err := storeChannel(id, sender.State); err != nil {
			return err
		}
	}

	rawTx, err := sender.Refund()
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", hex.EncodeToString(rawTx))

	return nil
}

func list(args []string) error {
	fmt.Printf("ID\tDomain\tStatus\tCapacity\tBalance\n")
	for id, c := range globalState.Channels {
		total := float64(c.State.FundingAmount) / 1e8
		balance := float64(c.State.Balance) / 1e8
		fmt.Printf("%s\t%s\t%s\t%.8f\t%.8f\n",
			id, c.Domain, c.State.Status, total, balance)
	}
	return nil
}

func show(args []string) error {
	id := args[0]
	c, ok := globalState.Channels[id]
	if !ok {
		return errors.New("not found")
	}
	buf, _ := json.MarshalIndent(c, "", "    ")
	fmt.Printf("%s\n", string(buf))
	return nil
}

func outputError(err string) {
	fmt.Printf("%v\n", err)
	os.Exit(1)
}

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		outputError("missing command")
		return
	}
	action := args[0]
	args = args[1:]

	net := getNet()

	s, err := load(net)
	if err != nil {
		outputError(err.Error())
	}
	globalState = s

	err = errors.New("unknown command")
	switch action {
	case "create":
		err = create(args)
	case "fund":
		err = fund(args)
	case "send":
		err = send(args)
	case "close":
		err = closeAction(args)
	case "refund":
		err = refund(args)
	case "list":
		err = list(args)
	case "show":
		err = show(args)
	}
	if err == nil {
		err = save(net, globalState)
	}
	if err != nil {
		outputError(err.Error())
	}
}

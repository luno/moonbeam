package main

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"

	"moonchan/channels"
	"moonchan/client"
	"moonchan/models"
)

var host = flag.String("host", "http://127.0.0.1:3211", "Server host")

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

func getNet() *chaincfg.Params {
	return &chaincfg.TestNet3Params
}

func loadkey() (*btcec.PrivateKey, *btcutil.AddressPubKey, error) {
	net := getNet()

	const wifstr = "cRTgZtoTP8ueH4w7nob5reYTKpFLHvDV9UfUfa67f3SMCaZkGB6L"
	wif, err := btcutil.DecodeWIF(wifstr)
	if err != nil {
		return nil, nil, err
	}

	pk := (*btcec.PublicKey)(&wif.PrivKey.PublicKey)
	pubkey, err := btcutil.NewAddressPubKey(pk.SerializeCompressed(), net)
	if err != nil {
		return nil, nil, err
	}

	return wif.PrivKey, pubkey, nil
}

func getClient() *client.Client {
	return client.NewClient(*host)
}

func create() error {
	privkey, _, err := loadkey()
	if err != nil {
		return err
	}
	net := getNet()

	s, err := channels.OpenChannel(net, privkey)
	if err != nil {
		return err
	}

	c := getClient()
	var req models.CreateRequest
	req.SenderPubKey = s.State.SenderPubKey.PubKey().SerializeCompressed()
	resp, err := c.Create(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	receiverPubKey, err := btcutil.NewAddressPubKey(resp.ReceiverPubKey, net)
	if err != nil {
		return err
	}

	s.ReceivedPubKey(receiverPubKey)

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

	globalState.Channels[resp.ID] = *ss

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

	sender, err := getChannel(id)
	if err != nil {
		return err
	}

	sig, err := sender.FundingTxMined(txid, uint32(vout), amount, height)
	if err != nil {
		return err
	}

	c := getClient()
	req := models.OpenRequest{
		ID:        id,
		TxID:      txid,
		Vout:      uint32(vout),
		SenderSig: sig,
	}
	resp, err := c.Open(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	return storeChannel(id, sender)
}

func send(args []string) error {
	id := args[0]
	amount, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return errors.New("invalid amount")
	}

	sender, err := getChannel(id)
	if err != nil {
		return err
	}

	sig, err := sender.PrepareSend(amount)
	if err != nil {
		return err
	}

	c := getClient()
	req := models.SendRequest{
		ID:        id,
		Amount:    amount,
		SenderSig: sig,
	}
	resp, err := c.Send(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	return storeChannel(id, sender)
}

func closeAction(args []string) error {
	id := args[0]

	sender, err := getChannel(id)
	if err != nil {
		return err
	}

	c := getClient()
	req := models.CloseRequest{
		ID: id,
	}
	resp, err := c.Close(req)
	output(req, resp, err)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", hex.EncodeToString(resp.CloseTx))

	return storeChannel(id, sender)
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

	s, err := load()
	if err != nil {
		outputError(err.Error())
	}
	globalState = s

	err = errors.New("unknown command")
	switch action {
	case "create":
		err = create()
	case "fund":
		err = fund(args)
	case "send":
		err = send(args)
	case "close":
		err = closeAction(args)
	}
	if err == nil {
		err = save(globalState)
	}
	if err != nil {
		outputError(err.Error())
	}
}

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"

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

func loadkey() (*btcec.PrivateKey, *btcutil.AddressPubKey, error) {
	net := &chaincfg.TestNet3Params

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

func create() error {
	pubkey, _, err := loadkey()
	if err != nil {
		return err
	}

	c := client.NewClient(*host)
	var req models.CreateRequest
	req.SenderPubKey = pubkey.PubKey().SerializeCompressed()
	resp, err := c.Create(req)
	return output(req, resp, err)
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

	err := errors.New("unknown command")
	switch args[0] {
	case "create":
		err = create()
	}
	if err != nil {
		outputError(err.Error())
	}
}

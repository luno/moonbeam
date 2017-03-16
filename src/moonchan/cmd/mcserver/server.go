package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"

	"moonchan/receiver"
)

func loadkey() (*btcec.PrivateKey, *btcutil.AddressPubKey, error) {
	net := &chaincfg.TestNet3Params

	const wifstr = "cUkJhR6V9Gjrw1enLJ7AHk37Bhtmfk3AyWkRLVhvHGYXSPj3mDLq"
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

func wrap(s *receiver.Receiver, h func(*receiver.Receiver, http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		h(s, w, r)
	}
}

func main() {
	flag.Parse()

	privKey, _, err := loadkey()
	if err != nil {
		log.Fatal(err)
	}

	s := receiver.NewReceiver(&chaincfg.TestNet3Params, privKey)

	http.HandleFunc("/", wrap(s, indexHandler))

	http.HandleFunc("/api/create", wrap(s, rpcCreateHandler))

	log.Fatal(http.ListenAndServe(":3211", nil))
}

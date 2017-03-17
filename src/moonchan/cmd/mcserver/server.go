package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcrpcclient"
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

func bitcoinClient() (*btcrpcclient.Client, error) {
	connCfg := &btcrpcclient.ConnConfig{
		Host:         "localhost:18332",
		User:         "username",
		Pass:         "password",
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	bc, err := btcrpcclient.New(connCfg, nil)
	if err != nil {
		return nil, err
	}

	blockCount, _ := bc.GetBlockCount()
	log.Printf("Connected to bitcoind. Block count = %d", blockCount)

	return bc, nil
}

type ServerState struct {
	PrivKey  *btcec.PrivateKey
	BC       *btcrpcclient.Client
	Receiver *receiver.Receiver
}

func wrap(s *ServerState, h func(*ServerState, http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
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

	bc, err := bitcoinClient()
	if err != nil {
		log.Fatal(err)
	}
	defer bc.Shutdown()

	s := receiver.NewReceiver(&chaincfg.TestNet3Params, privKey, bc)

	ss := &ServerState{privKey, bc, s}

	http.HandleFunc("/", wrap(ss, indexHandler))

	http.HandleFunc("/api/create", wrap(ss, rpcCreateHandler))
	http.HandleFunc("/api/open", wrap(ss, rpcOpenHandler))
	http.HandleFunc("/api/send", wrap(ss, rpcSendHandler))
	http.HandleFunc("/api/close", wrap(ss, rpcCloseHandler))

	log.Fatal(http.ListenAndServe(":3211", nil))
}

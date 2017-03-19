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

var testnet = flag.Bool("testnet", true, "Use testnet")
var destination = flag.String("destination", "mnRYb3Zpn6CUR9TNDL6GGGNY9jjU1XURD5", "Destination address")
var privkey = flag.String("privkey", "cUkJhR6V9Gjrw1enLJ7AHk37Bhtmfk3AyWkRLVhvHGYXSPj3mDLq", "WIF private key")
var bitcoindHost = flag.String("bitcoind_host", "localhost:18332", "")
var bitcoindUsername = flag.String("bitcoind_username", "username", "")
var bitcoindPassword = flag.String("bitcoind_password", "password", "")

func getnet() *chaincfg.Params {
	if *testnet {
		return &chaincfg.TestNet3Params
	}
	return &chaincfg.MainNetParams
}

func loadkey(net *chaincfg.Params) (*btcec.PrivateKey, *btcutil.AddressPubKey, error) {
	wif, err := btcutil.DecodeWIF(*privkey)
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
		Host:         *bitcoindHost,
		User:         *bitcoindUsername,
		Pass:         *bitcoindPassword,
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

	net := getnet()

	privKey, _, err := loadkey(net)
	if err != nil {
		log.Fatal(err)
	}

	bc, err := bitcoinClient()
	if err != nil {
		log.Fatal(err)
	}
	defer bc.Shutdown()

	s := receiver.NewReceiver(net, privKey, bc, *destination)

	go s.WatchBlockchainForever()

	ss := &ServerState{privKey, bc, s}

	http.HandleFunc("/", wrap(ss, indexHandler))
	http.HandleFunc("/details", wrap(ss, detailsHandler))
	http.HandleFunc("/close", wrap(ss, closeHandler))

	http.HandleFunc("/api/create", wrap(ss, rpcCreateHandler))
	http.HandleFunc("/api/open", wrap(ss, rpcOpenHandler))
	http.HandleFunc("/api/send", wrap(ss, rpcSendHandler))
	http.HandleFunc("/api/close", wrap(ss, rpcCloseHandler))

	log.Fatal(http.ListenAndServe(":3211", nil))
}

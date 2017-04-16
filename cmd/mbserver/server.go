package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcrpcclient"
	"github.com/btcsuite/btcutil/hdkeychain"

	"bitbucket.org/bitx/moonchan/receiver"
	"bitbucket.org/bitx/moonchan/resolver"
	"bitbucket.org/bitx/moonchan/storage/filesystem"
)

var testnet = flag.Bool("testnet", true, "Use testnet")
var destination = flag.String("destination", "mnRYb3Zpn6CUR9TNDL6GGGNY9jjU1XURD5", "Destination address")
var xprivkey = flag.String("xprivkey", "tprv8ZgxMBicQKsPe4s4h67jp6E3zhvfLRU6gnfrHRiwdfL3dR6AWJCw8sCiiGDVM4Nvw3muHfsdfbWVZwDi5TdiwiHrfYDXxGrfRFoYtdF2vnb", "Key chain extended private key")
var bitcoindHost = flag.String("bitcoind_host", "localhost:18332", "")
var bitcoindUsername = flag.String("bitcoind_username", "username", "")
var bitcoindPassword = flag.String("bitcoind_password", "password", "")
var listenAddr = flag.String("listen", ":3211", "Address to listen on")
var externalURL = flag.String("external_url", "https://example.com:3211", "External server URL")
var domain = flag.String("domain", "example.com", "Domain to accept payments for")
var tlsCert = flag.String("tls_cert", "tls/cert.pem", "TLS certificate")
var tlsKey = flag.String("tls_key", "tls/key.pem", "TLS key")
var authToken = flag.String("auth_token", "38a9cba31aed7e655b8d6d7014efc9bbc8ed9a961b708e90dc05e3b70994c5df", "Secret used to issue auth tokens")

func getnet() *chaincfg.Params {
	if *testnet {
		return &chaincfg.TestNet3Params
	}
	return &chaincfg.MainNetParams
}

func loadkey(net *chaincfg.Params) (*hdkeychain.ExtendedKey, error) {
	ek, err := hdkeychain.NewKeyFromString(*xprivkey)
	if err != nil {
		return nil, err
	}

	if !ek.IsForNet(net) {
		return nil, errors.New("xprivkey is for wrong network")
	}

	return ek, nil
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

	ek, err := loadkey(net)
	if err != nil {
		log.Fatal(err)
	}

	path := fmt.Sprintf("mbserver-state.%s.json", net.Name)
	storage := filesystem.NewFilesystemStorage(path)

	bc, err := bitcoinClient()
	if err != nil {
		log.Fatal(err)
	}
	defer bc.Shutdown()

	dir := receiver.NewDirectory(*domain)
	s := receiver.NewReceiver(net, ek, bc, storage, dir, *destination, *authToken)

	go s.WatchBlockchainForever()

	ss := &ServerState{bc, s}

	http.HandleFunc("/", wrap(ss, indexHandler))
	http.HandleFunc("/details", wrap(ss, detailsHandler))
	http.HandleFunc("/close", wrap(ss, closeHandler))

	if *externalURL != "" {
		http.HandleFunc(resolver.MoonbeamPath, domainHandler)
	}

	http.HandleFunc(rpcPath, wrap(ss, rpcHandler))
	http.HandleFunc(rpcPath+"/", wrap(ss, rpcHandler))

	fullAddr := *listenAddr
	if strings.HasPrefix(fullAddr, ":") {
		fullAddr = "127.0.0.1" + fullAddr
	}
	log.Printf("Listening on https://%s", fullAddr)

	if *tlsCert == "" {
		log.Fatal(http.ListenAndServe(*listenAddr, nil))
	} else {
		log.Fatal(http.ListenAndServeTLS(*listenAddr, *tlsCert, *tlsKey, nil))
	}
}

package main

import (
	"bytes"
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

	"moonchan/address"
	"moonchan/channels"
	"moonchan/client"
	"moonchan/models"
	"moonchan/resolver"
)

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

func getClient(id string) (*client.Client, error) {
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
	c, err := client.NewClient(httpClient, host)
	if err != nil {
		return err
	}
	var req models.CreateRequest
	req.SenderPubKey = s.State.SenderPubKey
	req.SenderOutput = s.State.SenderOutput
	resp, err := c.Create(req)
	if err != nil {
		return err
	}

	if !models.ValidateChannelID(resp.ID) {
		return errors.New("invalid channel ID")
	}

	receiverPubKey, err := btcutil.NewAddressPubKey(resp.ReceiverPubKey, net)
	if err != nil {
		return err
	}
	receiverPubKeyBytes := receiverPubKey.PubKey().SerializeCompressed()

	err = s.ReceivedPubKey(receiverPubKeyBytes, resp.ReceiverOutput, resp.Timeout, resp.Fee)
	if err != nil {
		return err
	}

	_, addr, err := s.State.GetFundingScript()
	if err != nil {
		return err
	}

	fmt.Printf("Funding address: %s\n", addr)

	// Sanity check to make sure client and server both agree on the state.
	if addr != resp.FundingAddress {
		return errors.New("state discrepancy")
	}

	if hasRemoteID(domain, resp.ID) {
		return errors.New("reused channel id")
	}

	id := strconv.Itoa(n)
	globalState.Channels[id] = Channel{
		Domain:   domain,
		Host:     host,
		State:    s.State,
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

	c, err := getClient(id)
	if err != nil {
		return err
	}
	req := models.OpenRequest{
		ID:        ch.RemoteID,
		TxID:      txid,
		Vout:      uint32(vout),
		SenderSig: sig,
	}
	if _, err := c.Open(req); err != nil {
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
		_, domain, valid := address.Decode(target)
		if !valid {
			return errors.New("invalid address")
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

	p := models.Payment{
		Amount: amount,
		Target: target,
	}
	payment, err := json.Marshal(p)
	if err != nil {
		return err
	}

	if _, err := sender.PrepareSend(p.Amount, payment); err != nil {
		return err
	}

	if ch.PendingPayment != nil {
		return errors.New("there is already a pending payment")
	}

	c, err := getClient(id)
	if err != nil {
		return err
	}
	req := models.ValidateRequest{
		ID:      ch.RemoteID,
		Payment: payment,
	}
	resp, err := c.Validate(req)
	if err != nil {
		return err
	}
	if !resp.Valid {
		return errors.New("payment rejected by server")
	}

	if err := storePendingPayment(id, sender.State, payment); err != nil {
		return err
	}
	if err := save(getNet(), globalState); err != nil {
		return err
	}

	return flush(id)
}

func flush(id string) error {
	ch, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	payment := ch.PendingPayment
	if payment == nil {
		fmt.Println("No pending payment to flush.")
		return nil
	}

	var p models.Payment
	if err := json.Unmarshal(payment, &p); err != nil {
		return err
	}

	sig, err := sender.PrepareSend(p.Amount, payment)
	if err != nil {
		return err
	}

	// Either the payment has been sent or it hasn't. Find out which one.

	c, err := getClient(id)
	if err != nil {
		return err
	}
	req := models.StatusRequest{
		ID: ch.RemoteID,
	}
	resp, err := c.Status(req)
	if err != nil {
		return err
	}

	serverBal := resp.Balance

	if serverBal == sender.State.Balance {
		// Pending payment doesn't reflect yet. We have to retry.

		req := models.SendRequest{
			ID:        ch.RemoteID,
			Payment:   payment,
			SenderSig: sig,
		}
		if _, err := c.Send(req); err != nil {
			return err
		}

		if err := sender.SendAccepted(p.Amount, payment); err != nil {
			return err
		}

		return storePendingPayment(id, sender.State, nil)

	} else if serverBal == sender.State.Balance+p.Amount {
		// Pending payment reflects. Finalize our side.

		if err := sender.SendAccepted(p.Amount, payment); err != nil {
			return err
		}

		return storePendingPayment(id, sender.State, nil)

	} else {
		return errors.New("unexpected remote channel balance")
	}
}

func flushAction(args []string) error {
	return flush(args[0])
}

func closeAction(args []string) error {
	id := args[0]

	ch, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	c, err := getClient(id)
	if err != nil {
		return err
	}
	req := models.CloseRequest{
		ID: ch.RemoteID,
	}
	resp, err := c.Close(req)
	if err != nil {
		return err
	}

	if err := sender.CloseReceived(resp.CloseTx); err != nil {
		return err
	}

	fmt.Printf("%s\n", hex.EncodeToString(resp.CloseTx))

	return storeChannel(id, sender.State)
}

func isClosing(s channels.Status) bool {
	return s == channels.StatusClosing || s == channels.StatusClosed
}

func status(args []string) error {
	id := args[0]

	ch, sender, err := getChannel(id)
	if err != nil {
		return err
	}

	c, err := getClient(id)
	if err != nil {
		return err
	}
	req := models.StatusRequest{
		ID: ch.RemoteID,
	}
	resp, err := c.Status(req)
	if err != nil {
		return err
	}

	buf, _ := json.MarshalIndent(resp, "", "    ")
	fmt.Printf("%s\n", string(buf))

	serverStatus := channels.Status(resp.Status)

	if sender.State.Status != serverStatus {
		fmt.Printf("Warning: Status differs\n")
	}
	if sender.State.Balance != resp.Balance {
		fmt.Printf("Warning: Balance differs\n")
	}
	if !bytes.Equal(sender.State.PaymentsHash[:], resp.PaymentsHash) {
		fmt.Printf("Warning: PaymentsHash differs\n")
	}

	if isClosing(serverStatus) && !isClosing(sender.State.Status) {
		if err := sender.Close(); err != nil {
			return err
		}
		return storeChannel(id, sender.State)
	}

	return nil
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
	all := false
	if len(args) > 0 && args[0] == "-a" {
		all = true
	}

	fmt.Printf("ID\tDomain\tStatus\tCapacity\tBalance\n")
	for id, c := range globalState.Channels {
		if c.State.Status != channels.StatusOpen && !all {
			continue
		}
		total := float64(c.State.Capacity) / 1e8
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

func help(args []string) error {
	fmt.Printf("Available commands:\n")
	for action, _ := range commands {
		h := helps[action]
		fmt.Printf("%10s %s\n", action, h)
	}
	return nil
}

func outputError(err string) {
	fmt.Printf("%v\n", err)
	os.Exit(1)
}

var commands = map[string]func(args []string) error{
	"create": create,
	"fund":   fund,
	"send":   send,
	"close":  closeAction,
	"refund": refund,
	"list":   list,
	"show":   show,
	"status": status,
	"flush":  flushAction,
}

var helps = map[string]string{
	"create": "Create a channel to a remote server",
	"fund":   "Open a created channel after funding transaction is confirmed",
	"send":   "Send a payment",
	"close":  "Close a channel",
	"refund": "Show the refund transaction for a channel",
	"list":   "List channels",
	"show":   "Show info about a channel",
	"status": "Get status from server",
	"flush":  "Flush any pending payment",
	"help":   "Show help",
}

func main() {
	flag.Parse()

	args := flag.Args()

	action := ""
	if len(args) == 0 {
		action = "help"
	} else {
		action = args[0]
		args = args[1:]
	}

	commands["help"] = help

	f, ok := commands[action]
	if !ok {
		outputError("unknown command")
		return
	}

	net := getNet()

	s, err := load(net)
	if err != nil {
		outputError(err.Error())
	}
	globalState = s

	err = f(args)
	if err == nil {
		err = save(net, globalState)
	}
	if err != nil {
		outputError(err.Error())
	}
}

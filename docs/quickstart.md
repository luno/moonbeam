# Quickstart

## Installation

You must first [download and install Go](https://golang.org/dl/) if you don't
have it already.

```bash
git clone git@github.com:luno/moonbeam.git
cd moonbeam
source ./vars.sh
go get github.com/btcsuite/btcutil
go get github.com/btcsuite/btcrpcclient
go install github.com/luno/moonbeam/cmd/mbclient
go install github.com/luno/moonbeam/cmd/mbserver
```

## Client Guide

The reference client is a standalone command-line program and doesn't require
direct access to a bitcoin daemon. It stores its state in a file called
`mbclient-state.{mainnet,testnet3}.json`. This file includes a private key so
should be kept safe and be backed-up.


### Initiate the channel

First we need to initiate a channel opening to a remote server. In this case,
https://bitcoinmoonbeam.org:

```bash
./bin/mbclient create bitcoinmoonbeam.org <refundaddr>
```

`refundaddr` is your own wallet address where the balance of coins will be
sent when the channel is eventually closed. Note that bitcoinmoonbeam.org is
running on testnet, so this must be a regular Bitcoin testnet address.

The funding address and channel ID are printed if successful.

To see the channel info, you can run:

```bash
./bin/mbclient list -a
```

To see the channel info on the server, visit https://bitcoinmoonbeam.org

### Fund the channel

Now you need to send some coins to the funding address. You can send any amount
larger than the channel fee. This amount will be the maximum channel capacity.
You can send it from any wallet.

You must wait for the transaction to confirm before proceeding.
Once the transaction has confirmed run:

```bash
./bin/mbclient fund <id> <txid> <vout> <amount_in_satoshi>
```

* `id` is the ID of the channel, you can find it by running `bin/mbclient list -a`
* `txid` is the ID of the funding Bitcoin transaction
* `vout` denotes the output of the transaction that contains the channel funding address, e.g. `0` for the first output
* `amount_in_satoshi` is the number of satoshis (1 satoshi = 0.00000001 bitcoin) to send

If successful, the channel is now open. To see your open channels, run:

```bash
./bin/mbclient list
```

To get more info about this particular channel, run:

```bash
./bin/mbclient show <id>
```

### Send payments

Now that the channel is open, you can send payments:

```bash
./bin/mbclient send mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb5Ap5B@bitcoinmoonbeam.org 1000
```

This will send 1000 Satoshi to mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb5Ap5B@bitcoinmoonbeam.org.

You can see the payments on the server at: https://bitcoinmoonbeam.org

### Close the channel

Once you're done, you can close the channel:

```bash
./bin/mbclient close <id>
```

This will print the closure transaction. The server should submit it to the
network, but you can broadcast it yourself too.

## Server Guide

The reference server requires access to a bitcoin daemon via JSON-RPC.
It stores its stage in a file called `mbserver-state.{mainnet,testnet3}.json`.
You can configure the server through flags.

To start the server:

```bash
./bin/mbserver --destination=<refundaddr> --xprivkey=<your_xprivkey> --auth_token=<random_secret>
```

You can then view the server status by visting https://127.0.0.1:3211.
By default, a self-signed SSL certificate is used, which you'll have to bypass
in your browser in order to view the page.

The available configuration flags can be found by running

```bash
./bin/mbserver --help
```

To create a channel to your test server, run:

```bash
./bin/mbclient create https://127.0.0.1:3211/moonbeamrpc <refundaddr>
```

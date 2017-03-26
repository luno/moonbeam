Moonbeam
========

Installation
------------

```
git clone git@bitbucket.org:bitx/moonchan.git
cd moonchan
source ./vars.sh
go get github.com/btcsuite/btcutil
go get github.com/btcsuite/btcrpcclient
go install moonchan/cmd/mcclient
```

Client Guide
------------

The reference client is a standalone command-line program and doesn't require
direct access to a bitcoin daemon. It stores its state in a file called
`client-state.{mainnet,testnet3}.json`. This file includes a private key so
should be kept safe and be backed-up.


### Initiate the channel

First we need to initiate a channel opening to a remote server. In this case,
https://bitcoinmoonbeam.org:

```
./bin/mcclient create bitcoinmoonbeam.org <refundaddr>
```

`refundaddr` is your own wallet address where the balance of coins will be
sent when the channel is eventually closed.

The funding address and channel ID are printed if successful.

To see the channel info, you can run:

```
./bin/mcclient list -a
```

To see the channel info on the server, visit
https://bitcoinmoonbeam.org/channels

### Fund the channel

Now you need to send some coins to the funding address. You can send any amount
- whatever amount will be the maximum channel capacity. You can send it using
any wallet.

You must wait for the transaction to confirm before proceeding.
Once the transaction has confirmed run:

```
./bin/mcclient fund <id> <txid> <vout> <amount_in_satoshi>
```

If successful, the channel is now open. To see your open channels, run:

```
./bin/mcclient list
```

To get more info about this particular channel, run:

```
./bin/mcclient show <id>
```

### Send payments

Now that the channel is open, you can send payments:

```
./bin/mcclient send test@bitcoinmoonbeam.org 1000
```

This will send 1000 satoshi to test@bitcoinmoonbeam.org.

You can see the payments on the server at:
https://bitcoinmoonbeam.org/payments

### Close the channel

Once you're done, you can close the channel:

```
./bin/mcclient close <id>
```

This will print the closure transaction. The server should submit it to the
network, but you can broadcast it yourself too.

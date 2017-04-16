# Moonbeam specification

## Abstract

The Moonbeam system allows for off-chain payments between two untrusted parties using Bitcoin payment channels. The system is suited for use between multi-user platforms such as hosted wallets, exchanges and payment processors. It can be deployed on the Bitcoin network as is today.

## Status

Draft

## Introduction

Moonbeam builds on the concept of payment channels in Bitcoin.
It describes a system whereby clients can initiate payment channels to send
payments to servers. The server in this context would usually be a multi-user
platform so that the channel can be used to send payments to any of the users
on the platform.

## Conventions

This specification defines various data structures. These are described by
structs in the Go language syntax. These structures are serialized into
JSON format when used in the protocol.

## Definitions

<dl>
  <dt>Sender, Client</td>
  <dd>The party sending Bitcoin to the receiver.</dd>

  <dt>Receiver, Server</dt>
  <dd>The party receiving Bitcoin from the sender.</dd>
</dl>

## Address Format

Moonbeam addresses have the following format:

`<address>+mb<version><checksum>@<domain>`

**address** is a standard Bitcoin address (e.g. “mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2”).  
**domain** is a full qualified domain name (e.g. “example.com”).  
**version** consists of 1 character  
**checksum** consists of 5 characters

The version and checksum characters are computed using the base58 encoding scheme used for standard Bitcoin addresses. First the string is rearranged to `<address>+mb@<domain>` and interpreted as a byte array. Next this byte array is encoded according to the base58 algorithm with version=0x01. The first character becomes version and the last 4 characters becomes checksum.

Future versions of Moonbeam may use larger version numbers.

Example:
`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com`

## Domain Resolution

In order to send a payment to “mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com”, the sender must open a channel to example.com. The domain resolution procedure describes how to resolve “example.com” to a suitable endpoint for the RPC protocol.

If domain accepts Moonbeam payments, then the URL `https://<domain>/moonbeam.json` will contain a JSON document pointing to the Moonbeam RPC endpoints for the domain.

This URL should be fetched over HTTPS, ensuring that the remote server certificate is validated. The server may return one or more 301 or 302 redirects which must be followed. (For example, `https://example.com/moonbeam.json` -> `https://www.example.com/moonbeam.json`.)

The `moonbeam.json` document has this structure:

```go
type Domain struct {
	Receivers []DomainReceiver `json:"receivers"`
}

type DomainReceiver struct {
	URL string `json:"url"`
}
```

Example:
```json
{
  "receivers": [
    {
      "url": "https://mb1.example.com"
    },
    {
      "url": "https://mb2.example.com/myprefix"
    }
  ]
}
```

The moonbeam.json document points to one or more endpoint URLs. The sender should select one of these endpoints, and may select others if the first server is unavailable.

Endpoint URLs must begin with “https://” and must not have a trailing slash.

## Channel parameters and state

These values are shared between the sender and receiver.

### Parameters

<dl>
  <dt>timeout</dt>
  <dd>Integer number of blocks until the sender can send the refund transaction</dd>
  <dt>fee</dt>
  <dd>Integer number of Satoshis to pay the network fee for the closure transaction</dd>
  <dt>net</dt>
  <dd>Bitcoin network to use: "mainnet" or "testnet3"</dd>
</dl>

### Channel status

<dl>
  <dt>status</dt>
  <dd>
    The current state of the channel.
    <dl>
     <dt>CREATED = 1</dt>
     <dd>channel has been initiated but not yet open for payments</dd>
     <dt>OPEN = 2</dt>
     <dd>the funding transaction has been mined and the channel is open for payments</dd>
     <dt>CLOSING = 3</dt>
     <dd>the closure or refund transaction has been broadcast</dd>
     <dt>CLOSED = 4</dt>
     <dd>the closure or refund transaction has been mined</dd>
    </dl>
  </dd>
</dl>


### Channel setup

Sender and receiver public keys:
<dl>
  <dt>senderPubKey</dt>
  <dd>sender's public key</dd>
  <dt>receiverPubKey</dt>
  <dd>receiver's public key</dd>
</dl>

Output addresses:
<dl>
  <dt>senderOutput</dt>
  <dd>address to which the sender’s change will be sent</dd>
  <dt>receiverOutput</dt>
  <dd>address to which the receiver’s balance will be sent</dd>
</dl>

Funding transaction output details:
<dl>
  <dt>fundingTxID</dt>
  <dd>transaction ID of the confirmed funding transaction</dd>
  <dt>fundingVout</dt>
  <dd>index of the confirmed funding transaction output</dd>
  <dt>capacity</dt>
  <dd>integer number of Satoshi equal to the value of the funding transaction output</dd>
</dl>

### Dynamic state

These values are updated as payments are sent through the channel.

<dl>
  <dt>balance</dt>
  <dd>integer number of Satoshi assigned from the sender to the receiver, initially 0</dd>
  <dt>paymentsHash</dt>
  <dd>a hash of the details of all the payments that make up the balance, initially 32 zero bytes</dd>
  <dt>senderSig</dt>
  <dd>sender’s signature for the closure transaction</dd>
</dl>

### Constants

The values defined as constants. They can't be varied per channel.

<dl>
  <dt>protocolVersion = 1</dt>
  <dd>the protocol version</dd>
  <dt>dustThreshold = 546</dt>
  <dd>minimum number of Satoshis for closure transaction outputs</dd>
</dl>

## Transaction scripts

### Funding output P2SH public key script

The funding transaction is sent to the P2SH address of this script.
It allows the capital to be spent either a) immediately with agreement of both
the sender and receiver, or b) by the sender after a delay of *timeout* blocks.

```
OP_IF
Push 2
	Push <senderPubKey>
	Push <receiverPubKey>
	Push 2
	Push OP_CHECKMULTISIG
OP_ELSE
	Push <timeout>
	OP_CHECKSEQUENCEVERIFY
	OP_DROP
	OP_DUP
	OP_HASH160
	Push <hash160(senderPubKey)>
	OP_EQUALVERIFY
	OP_CHECKSIG
OP_ENDIF
```

### Closure transaction

The closure transaction is used to close the channel by sending the *balance*
to the receiver and the remaining capital as change to the sender. It must
be signed by both the sender and receiver. There is also an additional
null data output containing *paymentsHash* to identify the set of payments
being settled.

Input 1:
```
OP_FALSE
Push <senderSig>
Push <receiverSig>
OP_TRUE
Push <redeemScript>
```

Output 1:
Pay 0 Satoshi to a null data script with data
_protcolVersion_ (1 byte) + _paymentsHash_ (32 bytes)

Output 2:
Pay *balance* to address _receiverOutput_.

Output 3:
Pay _capacity - balance - fee_ to address _senderOutput_.


If an output amount is zero, that output is omitted.
If an output amount is less than the *dustThreshold*,
that output is omitted and the amount is added to the network fee.

### Refund transaction

The refund transaction is used by the sender to refund the capital if the
receiver fails to close the channel. It only becomes valid after the *timeout*
has elapsed.

Input 1:
The input sequence must be set to *timeout*.
```
Push <senderSig>
Push <senderPubKey>
OP_FALSE
Push <redeemScript>
```

Outputs:
Any


## Payments

Payment details are represented by this structure:

```go
type Payment struct {
	Amount int64  `json:"amount"`  // amount in Satoshis
	Target string `json:"target"`  // Moonbeam address
}
```

If a payment _p_ is accepted by the receiver, the channel dynamic state is updated as follows:

_balance_ ← _balance_ + _p_.Amount  
_paymentsHash_ ← SHA256(serialize(_p_), _paymentsHash_)

Note that since the serialization of _p_ is not unique, the raw serialized data received from the sender should be used.

Both the sender and receiver should store their entire history of payments in raw serialized form so that it’s possible to recompute the hash. This is useful if there is a dispute. The hashes can be compared to the null data output of the closure transaction to verify which list of payments is correct.

The maximum acceptable serialized payment is 2^16 - 1 = 65535 bytes.

## RPC Protocol

The channel is manipulated via HTTP requests from the client to the server. The requests are sent to routes rooted at the endpoint URL. The request and response bodies are JSON. HTTP 200 is returned on success. A non-200 response is returned on failure.

The operations and channel IDs are encoded into the URLs so that servers may
apply some filtering and rate limiting based on the URLs alone.

### Channel IDs

A channel is uniquely identified by its funding outpoint
(*fundingTxID*, *fundingVout*). A channel id consists of the string
*fundingTxID*-*fundingVout*.

### Create

Initiate a channel opening. This creates a channel in the CREATED state.

`POST <endpoint>/create`

```go
type CreateRequest struct {
	Version int    `json:"version"`
	Net     string `json:"net"`

	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`
}

type CreateResponse struct {
	Version int    `json:"version"`
	Net     string `json:"net"`
	Timeout int64  `json:"timeout"`
	Fee     int64  `json:"fee"`

	ReceiverPubKey []byte `json:"receiverPubKey"`
	ReceiverOutput string `json:"receiverOutput"`

	FundingAddress string `json:"fundingAddress"`

        ReceiverData []byte `json:"receiverData"`
}
```

### Open

After the funding transaction has been mined, this moves the channel to the OPEN state.

`PUT <endpoint>/open/<txid>-<vout>`

```go
type OpenRequest struct {
        ReceiverData []byte `json:"receiverData"`

        Version int    `json:"version"`
	Net     string `json:"net"`
	Timeout int64  `json:"timeout"`
	Fee     int64  `json:"fee"`

	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`

        ReceiverPubKey []byte `json:"receiverPubKey"`
	ReceiverOutput string `json:"receiverOutput"`

	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	SenderSig []byte `json:"senderSig"`
}

type OpenResponse struct {
	AuthToken string `json:"authToken"`
}
```

### Validate

Validate checks whether a payment would be accepted if it is sent.

```
PUT <endpoint>/validate/<txid>-<vout>
Authorization: Bearer <authToken>
```

```go
type ValidateRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

        Payment []byte `json:"payment"`
}

type ValidateResponse struct {
	Valid bool `json:"valid"`
}
```

### Send

Send a payment and update the channel balance.

```
POST <endpoint>/send/<txid>-<vout>
Authorization: Bearer <authToken>
```

```go
type SendRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	Payment []byte `json:"payment"`

	SenderSig []byte `json:"senderSig"`
}

type SendResponse struct {
}
```

Note: The sender shouldn’t rely on any error returned. See a later section for an example of an attack based on the server returning incorrect errors.

### Close

Request the server to close the connection.

```
DELETE <endpoint>/close/<txid>-<vout>
Authorization: Bearer <authToken>
```

```go
type CloseRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

type CloseResponse struct {
	CloseTx []byte `json:"closeTx"`
}
```

### Status

Get the channel status and balance.

```
GET <endpoint>/status/<txid>-<vout>
Authorization: Bearer <authToken>
```

```go
type StatusRequest struct {
	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`
}

type StatusResponse struct {
	Status       int    `json:"status"`
	Balance      int64  `json:"balance"`
	PaymentsHash []byte `json:"paymentsHash"`
}
```


## Flows


### Initiating a channel

When a sender decides to open a new channel to a given domain, it first follows
the domain resolution procedure described above to get the server's RPC endpoint
URL.

The client then generates a new *senderPubKey* (probably using an HD key chain)
and sends a CreateRequest message to the server.

The server returns a CreateResponse message with the channel parameters.
If these parameters are acceptable, the client can continue with the flow.
If not, it can abandon the process.

The client should recompute the funding address from the local state and
ensure that it matches CreateResponse.FundingAddress.

The channel now has status CREATED.

### Funding the channel

The client now sends a payment to *fundingAddress*. The amount sent becomes the
*capacity* of the channel. The client can choose the amount as needed for its
expected payment activities.

Once the funding transaction has confirmed, the client should send an
OpenRequest with the txid to open the channel. The server will validate that
the (txid, vout) output is unspent. The channel is now open.

The channel now has status OPEN.

### Sending a payment (simplified)

The client can now send payments through the channel to targets on the server.
This is done by populating a Payment struct, sending a ValidateRequest to
check that the server will accept the payment, and then sending a SendRequest
to actually send the payment.

The SendRequest contains the client's signature for the updated channel closure
transaction. The server will validate that the signature is valid before
accepting the payment.

### Sending a payment (full)

Sending a payment is straightforward in principle but it becomes more complicated when you consider failure scenarios: The Send RPC could fail and we can’t be sure whether the server has processed the new payment or not. It is further complicated by the fact that we can’t trust the error returned by the Send RPC since the server could return a “fake” error.

The following procedure should be used to send a payment:

1. The payment is defined in the Payment struct described above.
2. This struct is serialized into a serializedPayment blob.
3. The sender calls the Validate RPC call to verify that the server will accept the payment.
4. The receiver stores the payment in durable storage.
5. Loop:
    1. Call the Status RPC to get the server’s view of the channel state
    2. If the server’s balance = local balance:
        1. Call the Send RPC to send the payment
    3. If the server’s balance = local balance + payment amount
        1. Update the local state to include the payment
        2. Exit the loop

After the first Send RPC call, whether it succeeds or not, we are committed to sending the payment. If we are unable to send it successfully, the only other option is the close the channel. This is done to avoid an attack described later. The Validate RPC call is needed to prevent denial of service if one the sender’s users requests a payment to an invalid address.

Since the _senderSig_ encodes the current channel state, it is always safe to retry sending the payment without any risk of a double payment.

If we fail to send a payment and close the channel, after the closure transaction has been mined, we can check whether the payment was in fact accepted or not.

#### Example scenario of an attack where the sender might return misleading errors

Assume the receiver has an account with the sender.

1. Channel is opened
2. Send transaction (a) of 1000 => receiver returns success
3. Send transaction (b) of 2000 => receiver returns a fake failure
4. Send transaction (c) of 100 => receiver returns success
5. Close

The receiver has a choice of either publishing the closure transaction for either (a, b) or (a, c).

If the sender believes the errors returned by the receiver, it will assume (b) was unsuccessful and so would have only debited 1000+100=1100 of funds from its users.

The attacker empties out account (b) at the sender through other means and publishes the closure transaction for (a, b) to receive a total of 3000, which causes the sender to lose money.

Prevention:
Close the channel after any failed transaction (after retrying).

### Closure

Once the client has finished sending payments, it can send a CloseRequest
message to request the server to close the channel. The server will then
broadcast the latest closure transaction and return the raw transaction.
The client should broadcast the closure transaction too.

The receiver must not accept any further payments after sharing the closure
transaction with the sender. Otherwise the sender could publish the transaction.

The channel status is now CLOSING. Once the closure transaction is confirmed,
the channel status becomes CLOSED.

### Blockchain monitoring

Throughout this flow, the server must monitor the blockchain. If the block
height gets too close to channel timeout, the server must close the channel
by broadcasting the closure transaction. Failure to do this early enought risks
that the client broadcast the refund transaction.


## Security considerations

The server must never share the signed closure transaction with the receiver
until the channel is closed. Otherwise, the sender could keep a copy of a
previous closure transaction and broadcast it.

The server could also broadcast a previous version of the closure transaction.
This would revoke any payments made after that version. However, it only results
in less money transferred to the receiver so would generally not be in the
interest of the receiver to do. If it happens, the sender needs to check the
closure hash and revoke the unsent payments.

The receiver must always check that the signed closure transaction is considered
"standard" by the bitcoin network mempool rules. Otherwise, it may be unable
to broadcast it to close the channel.

All HTTP requests (domain resolution and RPC) must be done over TLS and the
server-side certificate must be validated.

All values transmitted between the sender and receiver must obviously be
validated and range checked.

## Risks

**Cost of capital:**
The sender must put up capital in advance for payments that
it intends to make. There is a cost to this since it becomes inaccessible
until the channel is closed.

**Refund delay:**
If the receiver disappears, the channel can't be closed immediately.
The sender has to wait for the *timeout* before the refund transaction can be
broadcast. If the *timeout* is large, this could capture a significant amount
of capital in limbo which could be disruptive for the sender.

**Block space congestion**
If the bitcoin network becomes congested, the block space fee rate could rise
so that the closure transaction fee is too low to be confirmed quickly.
The closure transaction could be delayed longer than the *timeout* and the
sender could then broadcast the refund transaction first (with a higher fee).
To mitigate this, server should close the channel well before the *timeout*.

**Miner collusion**
The sender could bribe miners to exclude the closure transaction and then mine
the refund transaction after the timeout. This could be bone by specifying a
huge fee in the refund transaction. Currently the mempool rules reject the
refund transaction before *timeout* has elapsed, but the sender could share it
directly with miners so that they know to rather skip the closure transaction
and wait for the big payout.

**Need to monitor blockchain**
The receive must monitor the blockchain and ensure that it broadcasts the
closure transaction before the *timeout*. If, for example, the server goes
offline for an extended period of time, it may miss the window. Therefore, the
server needs to be reliable and redundant.

**DNS hijacking**
If the receiver's DNS server is hijacked, an attacker could receive payments
to new channels that were intended for the real receiver. This is partially
mitigated by requiring SSL, but if DNS is hijacked, the attacker could quickly
obtain a valid SSL certificate from Let's Encrypt. A psossible mitigation would
be to require the server to prove ownership of the target address in
ValidateResponse before sending the payment.

**Risk of the future**
Payments are sent over the channel in real time but the channel is only
finalized some time in the future. Any number of events could occur after the
channel is open that could disrupt the final closure. For example, a blockchain
fork could render the closure transaction invalid, or changes to mempool
acceptance rules could disrupt closure transactions.

**Denial of service**
The server must expose its endpoint to the public. Therefore, it can be a target
of DDOS attacks. This may be mitigated by filtering, rate limiting and providing
multiple endpoints in moonbeam.json.

## Recommended parameters

**timeout:**
The sender wants a smaller timeout to reduce its risk of stuck capital if the
receiver disappears. The receiver wants a larger timeout to ensure it has
sufficient time for the closure transaction to confirm.
We recommend a *timeout* of 7 days and for the receiver to close the channel
after 24 hours. This provides sufficient time for the receiver to recover from
server outages and some time for network fees to correct after any short-term
transaction backlog.

**fee:**
The fee should be chosen to be higher than a typical network fee because of the
importance of the closure transaction. We recommend choosing the fee rate to be
50% higher than the fee rate that would be used for normal payments.


### Receiver policy parameters

These are parameters that aren't technically required to be shared for the
channel to operate. However, it can be helpful for the sender to know them
e.g. to avoid using more capital than necessary.

<dl>
  <dt>softTimeout</dt>
  <dd>block count after which the receiver will close the channel</dd>
  <dt>paymentsMaxCount</dt>
  <dd>maximum number of payments before the receiver will close the channel</dd>
  <dt>balanceMax</td>
  <dd>maximum balance after which the receiver will close the channel</dd>
  <dt>fundingMinConf</dt>
  <dd>minimum number of confirmations that the receiver will accept for the funding transaction</dd>
  <dt>paymentMinAmount</dt>
  <dd>minimum transaction amount that the receiver will accept</dd>
  <dt>paymentMaxAmount</dt>
  <dd>Maximum transaction amount that the receiver will accept. zero means there is no maximum.</dd>
</dl>

## Outstanding issues

- Min amount is the dust threshold, but receiver doesn’t want channels closed at dust threshold because it costs more to spend than it’s worth.
- Sender must certify the channel (txid, vout) to prove that channel was accepted by the domain
- Validate should prove ownership of address by signing a message to avoid domain takeover attacks


## References

- [BIP 112, 2015](https://github.com/bitcoin/bips/blob/master/bip-0112.mediawiki)
- [Deployable Lightning, 2015, Rusty Russell](https://github.com/ElementsProject/lightning/blob/master/doc/deployable-lightning.pdf)

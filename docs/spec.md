# Moonbeam specification

## Abstract

The Moonbeam system allows for off-chain payments between two untrusted parties using Bitcoin payment channels. The system is suited for use between “hosted” wallet services that serve many users. It can be deployed on the Bitcoin network as is today.

## Introduction





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
**checksum** consists of 5 character

The version and checksum characters are computed using the base58 encoding scheme used for standard Bitcoin addresses. First the string is rearranged to `<address>+mb@<domain>` and interpreted as a byte array. Next this byte array is encoded according to the base58 algorithm with version=0x01. The first character becomes version and the last 4 characters becomes checksum.

Future versions of Moonbeam may use larger version numbers.

Example:
`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com`

## Name Resolution

In order to send a payment to “mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com”, the sender must open a channel to example.com. The name resolution procedure describes how to resolve “example.com” to a suitable endpoint for the RPC protocol.

If domain accepts Moonbeam payments, then the URL `https://<domain>/moonbeam.json` will contain a JSON document pointing to the Moonbeam RPC endpoints for the domain.

This URL should be fetched over HTTPS, ensuring that the remote server certificate is validated. The server may return one or more 301 or 302 redirects which must be followed. (For example, `https://example.com/moonbeam.json` -> `https://www.example.com/moonbeam.json`.)

The `moonbeam.json` document has this structure:

```
type Domain struct {
	Receivers []DomainReceiver `json:"receivers"`
}

type DomainReceiver struct {
	URL string `json:"url"`
}
```

Example:
```
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
  <dd>integer number of blocks until the sender can send the refund transaction</dd>
  <dt>fee</dt>
  <dd>integer number of satoshis to pay the network fee for the closure transaction</dd>
</dl>

### Channel status

<dl>
  <dt>status</dt>
  <dd>The current state of the channel.</dd>
</dl>

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

### Channel setup

Sender and receiver public keys:
<dl>
  <dt>senderPubKey</dt>
  <dt>receiverPubKey</dt>
</dl>
New keys must be generated for each new channel.

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
  <dd>integer number of Satoshi assigned from the sender to the receiver</dd>
  <dt>paymentsHash</dt>
  <dd>a hash of the details of all the payments that make up the balance</dd>
  <dt>senderSig</dt>
  <dd>sender’s signature for the closure transaction</dd>
</dl>

The initial _balance_ is 0 and the initial _paymentsHash_ is 32 zero bytes.

## Transaction scripts

### Funding output P2SH public key script

The funding transaction is sent to the P2SH address of this script.

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

Input 1:
```
OP_FALSE
Push <senderSig>
Push <receiverSig>
OP_TRUE
Push <redeemScript>
```

Output 1:
Pay 1 Satoshi to a null data script with data 0x01 + _paymentsHash_

Output 2:
Pay balance to address _receiverOutput_.

Output 3:
Pay _capacity - balance - fee_ to address _senderOutput_.


If an output amount is zero, that output is omitted.
If an output amount is less than the dust threshold, that output is omitted and the amount is added to the network fee.

### Refund transaction

The input sequence must be equal to timeout.

Input 1:
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

```
type Payment struct {
	Amount int64  `json:"amount"`
	Target string `json:"target"`
}
```

If a payment _p_ is accepted by the receiver, the channel dynamic state is updated as follows:

_balance_ ← _balance_ + _p_.Amount  
_paymentsHash_ ← SHA256(serialize(_p_), _paymentsHash_)

Note that since the serialization of _p_ is not unique, the raw serialized data received from the sender should be used.

Both the sender and receiver should store their entire history of payments in raw serialized form so that it’s possible to recompute the hash. This is useful if there is a dispute. The hashes can be compared to the null data output of the closure transaction to verify which list of payments is correct.

## RPC Protocol

The channel is manipulated via HTTP requests from the client to the server. The requests are sent to the endpoint URL or endpoint/channel_id if they relate to a specific channel. The request and response bodies are JSON. HTTP 200 is returned on success. A non-200 response is returned on failure.

### Channel IDs

A channel ID consists of 1 to 64 characters from the set of [a-z], [A-Z], [0-9], hyphen (-) and underscore (_). This is the same character range as used by the “URL-safe” base64 variant.

### Create

Initiate a channel opening. This creates a channel in the CREATED state.

`POST <endpoint>`

```
type CreateRequest struct {
	SenderPubKey []byte `json:"senderPubKey"`
	SenderOutput string `json:"senderOutput"`
}

type CreateResponse struct {
	ID string `json:"id"`

	Timeout int64 `json:"timeout"`
	Fee     int64 `json:"fee"`

	ReceiverPubKey []byte `json:"receiverPubKey"`
	ReceiverOutput string `json:"receiverOutput"`

	FundingAddress string `json:"fundingAddress"`
}
```

### Open

After the funding transaction has been mined, this moves the channel to the OPEN state.

`PATCH <endpoint>/<channel_id>`

```
type OpenRequest struct {
	ID string `json:"id"`

	TxID string `json:"txid"`
	Vout uint32 `json:"vout"`

	SenderSig []byte `json:"senderSig"`
}

type OpenResponse struct {
}
```

### Validate

Validate checks whether a payment would be accepted if it is sent.

`PATCH <endpoint>/<channel_id>`

```
type ValidateRequest struct {
	ID string `json:"id"`

	Payment []byte `json:"payment"`
}

type ValidateResponse struct {
	Valid bool `json:"valid"`
}
```

### Send

Send a payment and update the channel balance.

`POST <endpoint>/<channel_id>`

```
type SendRequest struct {
	ID string `json:"id"`

	Payment []byte `json:"payment"`

	SenderSig []byte `json:"senderSig"`
}

type SendResponse struct {
}
```

Note: The sender shouldn’t rely on any error returned. See a later section for an example of an attack based on the server returning incorrect errors.

### Close

Request the server to close the connection.

`DELETE <endpoint>/<channel_id>`

```
type CloseRequest struct {
	ID string `json:"id"`
}

type CloseResponse struct {
	CloseTx []byte `json:"closeTx"`
}
```

### Status

Get the channel status and balance.

`GET <endpoint>/<channel_id>`

```
type StatusRequest struct {
	ID string `json:"id"`
}

type StatusResponse struct {
	Status       int    `json:"status"`
	Balance      int64  `json:"balance"`
	PaymentsHash []byte `json:"paymentsHash"`
}
```


## Flow



## Sending a payment

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
    4. Update the local state to include the payment
    5. Exit the loop

After the first Send RPC call, whether it succeeds or not, we are committed to sending the payment. If we are unable to send it successfully, the only other option is the close the channel. This is done to avoid an attack described later. The Validate RPC call is needed to prevent denial of service if one the sender’s users requests a payment to an invalid address.

Since the _senderSig_ encodes the current channel state, it is always safe to retry sending the payment without any risk of a double payment.

If we fail to send a payment and close the channel, after the closure transaction has been mined, we can check whether the payment was in fact accepted or not.

### Example scenario of an attack where the sender might return misleading errors

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


## Security

Receiver must not accept any further payments after sharing the closure transaction with the sender. Otherwise the sender could publish the transaction.
Risks



## Outstanding issues

- Min amount is the dust threshold, but receiver doesn’t want channels closed at dust threshold because it costs more to spend than it’s worth.
- Sender must certify the channel (txid, vout) to prove that channel was accepted by the domain
- Validate should prove ownership of address by signing a message to avoid domain takeover attacks



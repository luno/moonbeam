# Moonbeam overview

This article gives a high-level simplified overview of Moonbeam.

## Table of contents

   * [Moonbeam overview](#moonbeam-overview)
      * [Introduction](#introduction)
      * [Basic operation](#basic-operation)
      * [How a channel works](#how-a-channel-works)
      * [How domains work](#how-domains-work)
      * [More information](#more-information)

## Introduction

Moonbeam is a protocol that uses Bitcoin payment channels to facilitate instant off-chain payments between multi-user platforms.

Moonbeam doesn't require segwit or larger blocks, and can be deployed on the
Bitcoin network today.

Here is how Moonbeam differs from standard Bitcoin payments:

| Standard Bitcoin payments | Moonbeam payments |
| --- | --- |
| On-chain | Off-chain |
| Requires 30 minute block confirmations | Instant |
| Requires a fee per transaction | Zero fee per transaction (though platforms may charge) |
| Don’t require prior setup | Requires prior channel setup |
| Decentralized | Semi-decentralized |
| Better for larger payments | Better for smaller payments |
| Addresses look like: mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2 | Addresses look like: mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com |

You may have heard of payment channels before in the context of the Lightning Network. Moonbeam and Lightning both use payment channels to process Bitcoin payments off-chain but they have different design goals. While not directly comparable, it’s helpful to contrast them:

| Lightning | Moonbeam |
| --- | --- |
| Requires Segwit | Works today |
| Uses bidirectional channels | Uses single direction channels |
| Routes payments over multiple channels | Routes payments over a single channel |
| Uses a peer-to-peer overlay network | Uses standard internet infrastructure |
| More complicated system | Less complicated system |

Moonbeam is designed especially for use by multi-user platforms such hosted wallets, exchanges and payment processors. We envisage that these platforms will open Moonbeam channels between one another and route their users’ payments through them.

Even though Moonbeam is a point-to-point system, it is still designed to work in a decentralized way. An address in Moonbeam looks like this:

`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com`

The @example.com piece means that this address is handled by the example.com domain. Platforms can use these domain names to automatically open Moonbeam channels and send payments without any prior arrangements or agreements, as long as they implement the Moonbeam protocol.

Moonbeam is designed to be easily adopted by users. Moonbeam addresses contain standard Bitcoin addresses so that they can be trivially used with existing Bitcoin software. This also gives platforms flexibility in implementing Moonbeam (e.g. they can implement sending only, receiving only, both, or even selectively per-payment).

## Basic operation

Suppose there are two wallet platforms Asiawallet and Batwallet.
Alice, a customer of Asiawallet, wants to send a payment to Bob, a customer
of Batwallet.

Batwallet assigns an address
`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@batwallet.com` to Bob, and Bob
gives this address to Alice.

Alice now logs onto Asiawallet and requests to send 0.01 BTC to
`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@batwallet.com`.

Asiawallet looks as the address and sees the @batwallet.com domain. If
Asiawallet already has a Moonbeam channel open to Batwallet, the payment is
sent over this channel. If not, Asiawallet can send it directly to
`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2`. In either case, Alice and Bob quickly
conclude their payment.

If Asiawallet sees many customers sending to @batwallet.com domain addresses,
they will keep channels open with sufficient capacity to process the expected
volume of payments. If, on the other hand, @batwallet.com isn't very popular,
Asiawallet won't open channels and will just send the payments as normal Bitcoin
transactions.

## How a channel works

A payment channel is a simple form of a smart contract.
The sender commits a certain amount of capital upfront into a special multisig
address. The multisig address requires both the sender and receiver to agree
on how to split the capital. Initially the entire capital is assigned to the
sender and none to the receiver. However, whenever a payment is sent over the
channel, the split is updated to that the receiver is assigned the sum of
payments sent.

The sender and receiver need to agree on the precise rules of the system for
this to work. The [Moonbeam protocol specification](spec.md) defines these
rules.

## How domains work

Opening a point-to-point channel requires cooperation between the sender and
receiver. However, we want to avoid the need for platforms to manually
coordinate these channels. Instead, we uses the familiar domain name system
to allow platforms to automatically discover and initiate channels between
one another.

The Moonbeam protocol defines the procedure to start from a domain name
(e.g. batwallet.com), locate the appropriate server, and negotiate and open the
payment channel.

## More information

- [Moonbeam Repository](../)
- [Specification](spec.md)

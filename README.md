# Moonbeam

Moonbeam is a protocol built on top of Bitcoin that allows for instant
off-chain payments between untrusted parties using payment channels.

This repository contains the protocol specification, documentation and
reference implementation.

## Introduction

Moonbeam is designed especially for use by multi-user platforms such hosted
wallets, exchanges and payment processors. We envisage that these platforms
will open Moonbeam channels between one another and route their users’ payments
through them.

Even though Moonbeam is a point-to-point system, it is still designed to work
in a decentralized way. An address in Moonbeam looks like this:

`mgzdqkEjYEjR5QNdJxYFnCKZHuNYa5bUZ2+mb7vCiK@example.com`

The @example.com piece means that this address is handled by the example.com
domain. Platforms can use the domain name to automatically open Moonbeam
channels and send payments without any prior arrangements or agreements, as
long as they both implement the Moonbeam protocol.

Moonbeam is designed to be easily adopted by users. Moonbeam addresses contain
standard Bitcoin addresses so that they can be trivially used with existing
Bitcoin software. This also gives platforms flexibility in implementing
Moonbeam (e.g. they can implement sending only, receiving only, both, or even
selectively per-payment).

## Documentation

[Quickstart](docs/quickstart.md)

[Specification](docs/spec.md)

Go Package Documentation

Demo server (testnet): https://bitcoinmoonbeam.org

## Reference implementation

This repository contains a reference client and server implementation written in
Go. They can be used standalone but the packages in this repository are
primarily designed to be imported into larger applications.

## Status

The Moonbeam protocol and reference implementation are still experimental.
They may still undergo non-backwards compatible changes.
The reference implementation requires further hardening.

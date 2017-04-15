package receiver

import (
	"github.com/btcsuite/btcd/chaincfg"
)

type policy struct {
	SoftTimeout    int
	FundingMinConf int
}

var policies = map[string]policy{
	"mainnet": policy{
		SoftTimeout:    144,
		FundingMinConf: 3,
	},
	"testnet3": policy{
		SoftTimeout:    32,
		FundingMinConf: 1,
	},
}

func getPolicy(net *chaincfg.Params) policy {
	p, ok := policies[net.Name]
	if ok {
		return p
	} else {
		return policies["mainnet"]
	}
}

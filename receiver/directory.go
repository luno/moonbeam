package receiver

import (
	"github.com/luno/moonbeam/address"
)

// Directory provides access to the set of targets.
// For example, a hosted wallet will have a list of targets corresponding to
// user accounts.
type Directory struct {
	domain string
}

func NewDirectory(domain string) *Directory {
	return &Directory{domain}
}

func (d *Directory) HasTarget(target string) (bool, error) {
	_, domain, valid := address.Decode(target)
	if !valid {
		return false, nil
	}

	if domain != d.domain {
		return false, nil
	}

	return true, nil
}

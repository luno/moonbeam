package receiver

import (
	"log"
	"time"

	"bitbucket.org/bitx/moonchan/channels"
	"bitbucket.org/bitx/moonchan/models"
	"bitbucket.org/bitx/moonchan/storage"
)

func (r *Receiver) checkChannel(blockCount int64, rec storage.Record) error {
	s := rec.SharedState
	if s.Status != channels.StatusOpen {
		return nil
	}

	timeout := int64(r.getPolicy().SoftTimeout)
	if timeout < s.Timeout {
		timeout = s.Timeout / 2
	}
	cutoff := int64(s.BlockHeight) + timeout

	if blockCount < cutoff {
		return nil
	}

	log.Printf("Closing channel %s due to nearing timeout", rec.ID)

	req := models.CloseRequest{
		TxID: s.FundingTxID,
		Vout: s.FundingVout,
	}
	_, err := r.Close(req)
	return err
}

func (r *Receiver) watchBlockchain() error {
	blockCount, err := r.bc.GetBlockCount()
	if err != nil {
		return err
	}

	recs, err := r.db.List()
	if err != nil {
		return err
	}

	var anyErr error
	for _, rec := range recs {
		if err := r.checkChannel(blockCount, rec); err != nil {
			anyErr = err
		}
	}

	return anyErr
}

func (r *Receiver) WatchBlockchainForever() {
	for {
		if err := r.watchBlockchain(); err != nil {
			log.Printf("watchBlockchain error: %v", err)
		}
		time.Sleep(time.Minute)
	}
}

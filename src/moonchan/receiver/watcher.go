package receiver

import (
	"log"
	"time"

	"moonchan/channels"
	"moonchan/models"
	"moonchan/storage"
)

func (r *Receiver) checkChannel(blockCount int64, rec storage.Record) error {
	s := rec.SharedState
	if s.Status != channels.StatusOpen {
		return nil
	}

	cutoff := int64(s.BlockHeight) + s.Timeout - channels.CloseWindow

	if blockCount < cutoff {
		return nil
	}

	log.Printf("Closing channel %s due to nearing timeout", rec.ID)

	req := models.CloseRequest{ID: rec.ID}
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

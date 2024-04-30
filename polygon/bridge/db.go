package bridge

import (
	"context"
	"encoding/binary"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/accounts/abi"
	"github.com/ledgerwatch/erigon/polygon/heimdall"
	"github.com/ledgerwatch/erigon/rlp"
)

func rlpToEvent(raw rlp.RawValue, stateContract abi.ABI) (*heimdall.EventRecordWithTime, error) {
	return heimdall.UnpackEventRecordWithTime(stateContract, raw)
}

// GetLatestEventID the latest state sync event ID in given DB, 1 if DB is empty
// NOTE: Polygon sync events start at index 1
func GetLatestEventID(db kv.RoDB, stateContract abi.ABI) (uint64, error) {
	var eventID uint64
	err := db.View(context.Background(), func(tx kv.Tx) error {
		cursor, err := tx.Cursor(kv.PolygonBridge)
		if err != nil {
			return err
		}

		k, _, err := cursor.Last()
		if err != nil {
			return err
		}

		if len(k) == 0 {
			eventID = 0
			return nil
		}

		eventID = binary.BigEndian.Uint64(k)
		return nil
	})

	return eventID, err
}

func AddEvents(db kv.RwDB, events []*heimdall.EventRecordWithTime) error {
	return db.Update(context.Background(), func(tx kv.RwTx) error {
		for _, event := range events {
			v, err := rlp.EncodeToBytes(event)
			if err != nil {
				return err
			}

			k := make([]byte, 8)
			binary.BigEndian.PutUint64(k, event.ID)
			err = tx.Put(kv.PolygonBridge, k, v)
			if err != nil {
				return err
			}
		}

		return nil
	})
}

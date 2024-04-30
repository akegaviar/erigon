package bridge

import (
	"context"
	"fmt"
	"time"

	"github.com/ledgerwatch/log/v3"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon/accounts/abi"
	"github.com/ledgerwatch/erigon/node"
	"github.com/ledgerwatch/erigon/node/nodecfg"
	"github.com/ledgerwatch/erigon/polygon/heimdall"
)

type fetchSyncEventsType func(ctx context.Context, fromId uint64, to time.Time, limit int) ([]*heimdall.EventRecordWithTime, error)

type Bridge struct {
	DB            kv.RwDB
	log           log.Logger
	stateContract abi.ABI
	ready         bool

	fetchSyncEvents fetchSyncEventsType
}

func NewBridge(ctx context.Context, config *nodecfg.Config, name string, readonly bool, logger log.Logger, fetchSyncEvents fetchSyncEventsType, stateContract abi.ABI) (*Bridge, error) {
	// create new db
	db, err := node.OpenDatabase(ctx, config, kv.PolygonDB, name, readonly, logger)
	if err != nil {
		return nil, err
	}

	return &Bridge{
		DB:              db,
		log:             logger,
		stateContract:   stateContract,
		fetchSyncEvents: fetchSyncEvents,
	}, nil
}

func (b *Bridge) Run(ctx context.Context) error {
	// start syncing
	b.log.Debug(bridgeLogPrefix("Bridge is running"))

	// get last known sync ID
	lastEventID, err := GetLatestEventID(b.DB, b.stateContract)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// get all events from last sync ID to now
		to := time.Now()
		events, err := b.fetchSyncEvents(ctx, lastEventID+1, to, 0)
		if err != nil {
			return err
		}

		if len(events) != 0 {
			b.ready = false
			if err := AddEvents(b.DB, events); err != nil {
				return err
			}

			lastEventID = events[len(events)-1].ID
		} else {
			b.ready = true
			time.Sleep(30 * time.Second)
		}

		b.log.Debug(bridgeLogPrefix(fmt.Sprintf("got %v new events, last event ID: %v, ready: %v\n", len(events), lastEventID, b.Ready())))
	}
}

func (b *Bridge) Ready() bool {
	return b.ready
}

func (b *Bridge) Close() {
	b.DB.Close()
}

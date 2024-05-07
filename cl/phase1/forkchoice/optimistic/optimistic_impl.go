package optimistic

import (
	"sync"

	"github.com/ledgerwatch/erigon-lib/common"
	"github.com/ledgerwatch/erigon/cl/cltypes"
)

type optimisticStoreImpl struct {
	opMutex         sync.RWMutex
	optimisticRoots map[common.Hash]*opNode
}

func NewOptimisticStore() OptimisticStore {
	return &optimisticStoreImpl{
		optimisticRoots: make(map[common.Hash]*opNode),
	}
}

func (impl *optimisticStoreImpl) AddOptimisticCandidate(block *cltypes.BeaconBlock) error {
	root, err := block.HashSSZ()
	if err != nil {
		return err
	}
	parentRoot := block.ParentRoot

	impl.opMutex.Lock()
	defer impl.opMutex.Unlock()
	if _, ok := impl.optimisticRoots[root]; ok {
		// block already optimistically imported
		return nil
	}
	blockNode := &opNode{
		parent:   parentRoot,
		children: []common.Hash{},
	}
	impl.optimisticRoots[root] = blockNode

	// check if parent is already in the store
	if _, ok := impl.optimisticRoots[parentRoot]; ok {
		impl.optimisticRoots[parentRoot].children = append(impl.optimisticRoots[parentRoot].children, root)
	}
	return nil
}

func (impl *optimisticStoreImpl) ValidateBlock(block *cltypes.BeaconBlock) error {
	// When a block transitions from NOT_VALIDATED -> VALID, all ancestors of the block MUST also transition
	// from NOT_VALIDATED -> VALID. Such a block and any previously NOT_VALIDATED ancestors are no longer considered "optimistically imported".
	impl.opMutex.Lock()
	defer impl.opMutex.Unlock()
	curRoot := block.StateRoot
	for {
		if node, ok := impl.optimisticRoots[curRoot]; ok {
			// validate the block
			// remove the block from the store
			delete(impl.optimisticRoots, curRoot)
			curRoot = node.parent
		} else {
			break
		}
	}
	return nil
}

func (impl *optimisticStoreImpl) InvalidateBlock(block *cltypes.BeaconBlock) error {
	// When a block transitions from NOT_VALIDATED -> INVALIDATED, all descendants of the block MUST also transition
	// from NOT_VALIDATED -> INVALIDATED.
	impl.opMutex.Lock()
	defer impl.opMutex.Unlock()
	toRemoves := []common.Hash{block.StateRoot}
	for len(toRemoves) > 0 {
		curRoot := toRemoves[0]
		toRemoves = toRemoves[1:]
		if node, ok := impl.optimisticRoots[curRoot]; ok {
			// invalidate the block
			// remove the block from the store
			delete(impl.optimisticRoots, curRoot)
			toRemoves = append(toRemoves, node.children...)
		}
	}
	return nil
}

func (impl *optimisticStoreImpl) IsOptimistic(root common.Hash) bool {
	if root == (common.Hash{}) {
		return false
	}
	if _, ok := impl.optimisticRoots[root]; ok {
		return true
	}
	return false
}

type opNode struct {
	parent   common.Hash
	children []common.Hash
}

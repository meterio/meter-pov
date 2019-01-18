package pow

import (
        "fmt"
        "sync"
        "sync/atomic"
        "container/list"
	// "bytes"


        "github.com/ethereum/go-ethereum/common"
        "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
)


type HeaderMap interface {
        Reset()
        InsertHeader(header *types.Header) bool
        RemoveHeader(header *types.Header) 
	CheckDepth() uint64
}

// maintains a cache of transactions.
type powHeader struct {
        mtx  sync.Mutex
	maxHeight int   // the max height allowed in the map
        size int
        depth uint64       // the current depth of pow blocks
	height uint64     // the current highest height of the blocks
	rootheight uint64 // the height of the root
        map_ map[string]uint64
        list *list.List // to remove oldest tx when cache gets too big
        currentHeader atomic.Value
        currentHeaderHash common.Hash
	currentRoot atomic.Value // The current root ETH header in current round
}

// newMapTxCache returns a new mapHeader.
func newPowHeader(cacheSize int, mxHeight int) *powHeader {
        return &powHeader{
                size: cacheSize,
                maxHeight: mxHeight,
                map_: make(map[string]uint64, cacheSize),
                list: list.New(),
        }
}


// it takes the inputs from the raw tx received by POS, and format back to the
// ETH header as needed.
func ValidateHeaderRaw(tx []byte) (header *types.Header, err error) {

    powtx := &types.Header{}

    erro := rlp.DecodeBytes(tx, powtx)
    if (erro != nil) {
        fmt.Errorf(" Failed to Decode the input pow tx") 
        return nil, erro
    }
 
    // TODO validate the ETH header
    return powtx, nil
 
}

func (ht *powHeader) SetRoot (header *types.Header) {
    ht.currentRoot.Store(header) 
    ht.rootheight = header.Number.Uint64()
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (ht *powHeader) CurrentRoot() *types.Header {
	if (ht.currentRoot.Load() == nil) {
		return nil
	}

        return ht.currentRoot.Load().(*types.Header)
}
 
func (ht *powHeader) SetCurrentHeader (header *types.Header) {
    ht.currentHeader.Store(header)
}

// CurrentBlock retrieves the current head block of the canonical chain. The
// block is retrieved from the blockchain's internal cache.
func (ht *powHeader) CurrentHeader() *types.Header {
        return ht.currentHeader.Load().(*types.Header)
}


// InsertHeader writes a header into the local tree, given that its parent is
// already known. If the total difficulty of the newly inserted header becomes
// greater than the current known TD, the canonical chain is re-routed.
//
// Note: This method is not concurrent-safe with inserting blocks simultaneously
// into the chain, as side effects caused by reorganisations cannot be emulated
// without the real blocks. Hence, writing headers directly should only be done
// in two scenarios: pure-header mode of operation (light clients), or properly
// separated header/block phases (non-archive clients).
func (ht *powHeader) InsertHeader(header *types.Header) bool {
        ht.mtx.Lock()
        defer ht.mtx.Unlock()

        hash := header.Hash().Bytes()

        if _, exists := ht.map_[string(hash)]; exists {
                return false
        }

        if ht.list.Len() >= ht.size {
                popped := ht.list.Front()
                poppedHeader := popped.Value.(types.Header)
                delete(ht.map_, string(poppedHeader.Hash().Bytes()))
                ht.list.Remove(popped)
        }

        // finally a new header

	// Bootrap the node for nil root header
	// TODO, eventually the root need to be on the canonical chain
	if currentRoot := ht.CurrentRoot(); currentRoot == nil {
		ht.SetRoot(header)
		ht.map_[string(hash)] = header.Number.Uint64()
        	ht.list.PushBack(header)
		ht.depth = 1	
		return true
	}

	fmt.Println("Receiving header %ulong\n", header.Number.Uint64())
	// Make sure the new header has valid root or parents
        if (!ht.IsHeaderExist(header.ParentHash, (header.Number.Uint64() -1))) {
		fmt.Println("Wrong POW Header without valid parent %ulong", (header.Number.Uint64() -1))

		return false // TODO need to check the header and root
	}

	// store hash<>blocknumber
	// store full header
        ht.map_[string(hash)] = header.Number.Uint64() 
        ht.list.PushBack(header)
       
        // track the depth update only if the new Header has a new Height
	// Keep in mind, we do allow the block with the small or same height to 
	// add, but not update the depth
        if (ht.height < header.Number.Uint64()) {
		ht.height = header.Number.Uint64()
		ht.depth = ht.depth + 1
	}

	ht.SetCurrentHeader(header)

        return true
}

func (ht *powHeader) RemoveHeader(header *types.Header) {
        ht.mtx.Lock()
        delete(ht.map_, string(header.Hash().Bytes()))
	// TODO need to also remove from the list ??
        ht.mtx.Unlock()

}


// This is frequently called with each K-block
func (ht *powHeader) Reset() {
        ht.mtx.Lock()
        ht.map_ = make(map[string]uint64, ht.size)
	ht.height = 0
	ht.depth = 0
	ht.rootheight = 0
	ht.currentRoot.Store(0) 
        ht.list.Init()
        ht.mtx.Unlock()
}

func (ht *powHeader) CheckNonce(header types.Header) bool {
    return false 
}

// return the height of the current cache 
func (ht *powHeader) CheckHeight() uint64 {
    return ht.height 
}

// return the Depth of the current cache
func (ht *powHeader) CheckDepth() uint64 {
    return ht.depth
}

// return the height of the current cache
func (ht *powHeader) CheckRootHeight() uint64 {
    return ht.rootheight
}


// Find a pow header by Hash value and Height
func (ht *powHeader) FindHeader(h common.Hash, height uint64) *types.Header {
	ht.mtx.Lock()
	defer ht.mtx.Unlock()

	if value, ok := ht.map_[string(h.Bytes())]; ok {
		if value == height {
		    // ok, find one record in the map, retrieve the header now
		    for e := ht.list.Front(); e != nil; e = e.Next() {
                          header := e.Value.(types.Header)
                          if (header.Hash() == h && header.Number.Uint64() == height) {
				   newheader := header
                                   return &newheader
                          }
		    }
                }      
	} 
	return nil 
}

func (ht *powHeader) IsHeaderExist(h common.Hash, height uint64) bool {
        ht.mtx.Lock()
        defer ht.mtx.Unlock()

        if value, ok := ht.map_[string(h.Bytes())]; ok {
                if value == height {
                    // ok, find one record in the map
			return true
                }
        }
        return false
}


func (ht *powHeader) GetHighestHeader() *types.Header {
        ht.mtx.Lock()
        defer ht.mtx.Unlock()

       height := ht.CheckHeight()
       for key, value := range ht.map_ {
               if value == height {
                       // find one block match the depth
                       for e := ht.list.Front(); e != nil; e = e.Next() {
                               header := e.Value.(types.Header)
                               if (string(header.Hash().Bytes()) == key) {
				       newheader := header
                                       return &newheader 
                               }
                       }       
               }
       }
       return nil 
}

// This will find the pow header list from pow pool
func (ht *powHeader) SetNewChain () []*types.Header {
	ht.mtx.Lock()
        defer ht.mtx.Unlock()
	
        // make sure minimum depth. ok to be bigger than 10
        if (ht.depth < 10) {
                return nil
        }

	var list []*types.Header

        header := ht.GetHighestHeader()
        if (header == nil) {
                return nil
        }

        list = append(list, header)

        // add the parents to the list now till hit the root
	// The list with max be 10. In idea case, it would be exactly 10
	for (header != nil) {
        	header = ht.FindHeader(header.ParentHash, (header.Number.Uint64() - 1))
		if (header != nil) {
			currentRoot := ht.CurrentRoot()
			if (header == currentRoot) {
				// reach to the common point
				fmt.Println("Reach to the root header")
			        return list	
			}
			list = append(list, header)
		}
	}

	// Shouldn't reach to here
	fmt.Println("Get the pow chain reach to the nil header")

	return nil
}



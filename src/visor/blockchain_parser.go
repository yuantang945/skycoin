package visor

import (
	"fmt"

	"github.com/skycoin/skycoin/src/coin"
	"github.com/skycoin/skycoin/src/visor/historydb"
)

// ParserOption option type which will be used when creating parser instance
type ParserOption func(*BlockchainParser)

// BlockchainParser parses the blockchain and stores the data into historydb.
type BlockchainParser struct {
	historyDB *historydb.HistoryDB
	blkC      chan coin.Block
	closing   chan chan struct{}
	bc        *Blockchain

	isStart bool
}

// NewBlockchainParser create and init the parser instance.
func NewBlockchainParser(hisDB *historydb.HistoryDB, bc *Blockchain, ops ...ParserOption) *BlockchainParser {
	bp := &BlockchainParser{
		bc:        bc,
		historyDB: hisDB,
		closing:   make(chan chan struct{}),
		blkC:      make(chan coin.Block, 10),
	}

	for _, op := range ops {
		op(bp)
	}

	return bp
}

// BlockListener when new block appended to blockchain, this method will b invoked
func (bcp *BlockchainParser) BlockListener(b coin.Block) {
	bcp.blkC <- b
}

// Run starts blockchain parser
func (bcp *BlockchainParser) Run() error {
	logger.Info("Blockchain parser start")
	defer logger.Info("Blockchain parser closed")

	if err := bcp.historyDB.ResetIfNeed(); err != nil {
		return err
	}

	// parse to the blockchain head
	headSeq := bcp.bc.Head().Seq()
	if err := bcp.parseTo(headSeq); err != nil {
		return err
	}

	for {
		select {
		case cc := <-bcp.closing:
			cc <- struct{}{}
			return nil
		case b := <-bcp.blkC:
			if err := bcp.parseTo(b.Head.BkSeq); err != nil {
				return err
			}
		}
	}
}

// Stop close the block parsing process.
func (bcp *BlockchainParser) Stop() {
	cc := make(chan struct{}, 1)
	bcp.closing <- cc
	<-cc
}

func (bcp *BlockchainParser) parseTo(bcHeight uint64) error {
	parsedHeight := bcp.historyDB.ParsedHeight()

	for i := int64(0); i < int64(bcHeight)-parsedHeight; i++ {
		b := bcp.bc.GetBlockInDepth(uint64(parsedHeight + i + 1))
		if b == nil {
			return fmt.Errorf("no block exist in depth:%d", parsedHeight+i+1)
		}

		if err := bcp.historyDB.ProcessBlock(b); err != nil {
			return err
		}
	}

	return nil
}

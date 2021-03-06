package block

import (
	"encoding/json"
	"fmt"

	"github.com/btcsuite/btcutil/base58"

	"boscoin.io/sebak/lib/ballot"
	"boscoin.io/sebak/lib/common"
	"boscoin.io/sebak/lib/common/observer"
	"boscoin.io/sebak/lib/consensus/round"
	"boscoin.io/sebak/lib/error"
	"boscoin.io/sebak/lib/storage"
	"boscoin.io/sebak/lib/transaction"
)

const (
	maxBlockHeightStringLength int    = 20
	EventBlockPrefix           string = "bk-saved"
)

type Block struct {
	Header
	Transactions []string `json:"transactions"` /* []Transaction.GetHash() */
	//PrevConsensusResult ConsensusResult

	Hash      string      `json:"hash"`
	Confirmed string      `json:"confirmed"`
	Proposer  string      `json:"proposer"` /* Node.Address() */
	Round     round.Round `json:"round"`
}

func (bck Block) Serialize() (encoded []byte, err error) {
	encoded, err = json.Marshal(bck)
	return
}

func (bck Block) String() string {
	encoded, _ := json.MarshalIndent(bck, "", "  ")
	return string(encoded)
}

// MakeGenesisBlock makes genesis block from genesis account and transaction.
// The genesis block has different part from the other Block
// * `Block.Proposer` is empty
// * `Block.Round` is empty
// * `Block.Confirmed` is `common.GenesisBlockConfirmedTime`
// * has only one `Transaction`
//
// This Transaction is different from other normal Transaction;
// * must have only one `Operation`, `OperationCreateAccount`
// * `Transaction.B.Source` is same with `OperationCreateAccount.Target`
// * `Transaction.B.Fee` is 0
// * `OperationCreateAccount.Amount` is same with balance of genesis account
// * `OperationCreateAccount.Target` is genesis account
func MakeGenesisBlock(st *storage.LevelDBBackend, account BlockAccount, networdID []byte) (blk Block, err error) {
	var exists bool
	if exists, err = ExistsBlockByHeight(st, 1); exists || err != nil {
		if exists {
			err = errors.ErrorBlockAlreadyExists
		}

		return
	}

	// create create-account transaction.
	opb := transaction.NewOperationBodyCreateAccount(account.Address, account.Balance, "")
	op := transaction.Operation{
		H: transaction.OperationHeader{
			Type: transaction.OperationCreateAccount,
		},
		B: opb,
	}

	txBody := transaction.TransactionBody{
		Source:     account.Address,
		Fee:        0,
		SequenceID: account.SequenceID,
		Operations: []transaction.Operation{op},
	}

	tx := transaction.Transaction{
		T: "transaction",
		H: transaction.TransactionHeader{
			Created: common.GenesisBlockConfirmedTime,
			Hash:    txBody.MakeHashString(),
		},
		B: txBody,
	}
	tx.Sign(kp, []byte(networdID))

	transactions := []string{tx.GetHash()}

	blk = NewBlock(
		"",
		round.Round{}, // empty round
		transactions,
		common.GenesisBlockConfirmedTime,
	)
	if err = blk.Save(st); err != nil {
		return
	}

	raw, _ := tx.Serialize()
	bt := NewBlockTransactionFromTransaction(blk.Hash, blk.Height, blk.Confirmed, tx, raw)
	if err = bt.Save(st); err != nil {
		return
	}

	return
}

func NewBlock(proposer string, round round.Round, transactions []string, confirmed string) Block {
	b := &Block{
		Header:       *NewBlockHeader(round, uint64(len(transactions)), getTransactionRoot(transactions)),
		Transactions: transactions,
		Proposer:     proposer,
		Round:        round,
		Confirmed:    confirmed,
	}

	b.Hash = base58.Encode(common.MustMakeObjectHash(b))
	return *b
}

func NewBlockFromBallot(b ballot.Ballot) Block {
	return NewBlock(
		b.Proposer(),
		b.Round(),
		b.Transactions(),
		b.ProposerConfirmed(),
	)
}

func getTransactionRoot(txs []string) string {
	return common.MustMakeObjectHashString(txs) // TODO make root
}

func GetBlockKey(hash string) string {
	return fmt.Sprintf("%s%s", common.BlockPrefixHash, hash)
}

func GetBlockKeyPrefixConfirmed(confirmed string) string {
	return fmt.Sprintf("%s%s-", common.BlockPrefixConfirmed, confirmed)
}

func GetBlockKeyPrefixHeight(height uint64) string {
	f := fmt.Sprintf("%%s%%0%dd-", maxBlockHeightStringLength)
	return fmt.Sprintf(f, common.BlockPrefixHeight, height)
}

func (b Block) NewBlockKeyConfirmed() string {
	return fmt.Sprintf(
		"%s%s%s",
		GetBlockKeyPrefixConfirmed(b.Confirmed),
		common.EncodeUint64ToByteSlice(b.Height),
		common.GetUniqueIDFromUUID(),
	)
}

func (b Block) Save(st *storage.LevelDBBackend) (err error) {
	key := GetBlockKey(b.Hash)

	var exists bool
	exists, err = st.Has(key)
	if err != nil {
		return
	} else if exists {
		return errors.ErrorBlockAlreadyExists
	}

	if err = st.New(key, b); err != nil {
		return
	}

	if err = st.New(b.NewBlockKeyConfirmed(), b.Hash); err != nil {
		return
	}
	if err = st.New(GetBlockKeyPrefixHeight(b.Height), b.Hash); err != nil {
		return
	}

	observer.BlockObserver.Trigger(EventBlockPrefix, b)

	return
}

func GetBlock(st *storage.LevelDBBackend, hash string) (bt Block, err error) {
	err = st.Get(GetBlockKey(hash), &bt)
	return
}

func GetBlockHeader(st *storage.LevelDBBackend, hash string) (bt Header, err error) {
	err = st.Get(GetBlockKey(hash), &bt)
	return
}

func ExistsBlock(st *storage.LevelDBBackend, hash string) (exists bool, err error) {
	exists, err = st.Has(GetBlockKey(hash))
	return
}

func ExistsBlockByHeight(st *storage.LevelDBBackend, height uint64) (exists bool, err error) {
	exists, err = st.Has(GetBlockKeyPrefixHeight(height))
	return
}

func LoadBlocksInsideIterator(
	st *storage.LevelDBBackend,
	iterFunc func() (storage.IterItem, bool),
	closeFunc func(),
) (
	func() (Block, bool, []byte),
	func(),
) {

	return (func() (Block, bool, []byte) {
			item, hasNext := iterFunc()
			if !hasNext {
				return Block{}, false, item.Key
			}

			var hash string
			json.Unmarshal(item.Value, &hash)

			b, err := GetBlock(st, hash)
			if err != nil {
				return Block{}, false, item.Key
			}

			return b, hasNext, item.Key
		}), (func() {
			closeFunc()
		})
}

func LoadBlockHeadersInsideIterator(
	st *storage.LevelDBBackend,
	iterFunc func() (storage.IterItem, bool),
	closeFunc func(),
) (
	func() (Header, bool, []byte),
	func(),
) {

	return (func() (Header, bool, []byte) {
			item, hasNext := iterFunc()
			if !hasNext {
				return Header{}, false, []byte{}
			}

			var hash string
			json.Unmarshal(item.Value, &hash)

			b, err := GetBlockHeader(st, hash)
			if err != nil {
				return Header{}, false, []byte{}
			}

			return b, hasNext, item.Key
		}), (func() {
			closeFunc()
		})
}

func GetBlocksByConfirmed(st *storage.LevelDBBackend, options storage.ListOptions) (
	func() (Block, bool, []byte),
	func(),
) {
	iterFunc, closeFunc := st.GetIterator(common.BlockPrefixConfirmed, options)

	return LoadBlocksInsideIterator(st, iterFunc, closeFunc)
}

func GetBlockHeadersByConfirmed(st *storage.LevelDBBackend, options storage.ListOptions) (
	func() (Header, bool, []byte),
	func(),
) {
	iterFunc, closeFunc := st.GetIterator(common.BlockPrefixConfirmed, options)

	return LoadBlockHeadersInsideIterator(st, iterFunc, closeFunc)
}

func GetBlockByHeight(st *storage.LevelDBBackend, height uint64) (bt Block, err error) {
	var hash string
	if err = st.Get(GetBlockKeyPrefixHeight(height), &hash); err != nil {
		return
	}

	return GetBlock(st, hash)
}

func GetBlockHeaderByHeight(st *storage.LevelDBBackend, height uint64) (bt Header, err error) {
	var hash string
	if err = st.Get(GetBlockKeyPrefixHeight(height), &hash); err != nil {
		return
	}

	return GetBlockHeader(st, hash)
}

func GetLatestBlock(st *storage.LevelDBBackend) (b Block, err error) {
	// get latest blocks
	iterFunc, closeFunc := GetBlocksByConfirmed(st, storage.NewDefaultListOptions(true, nil, 1))
	b, _, _ = iterFunc()
	closeFunc()

	if b.Hash == "" {
		err = errors.ErrorBlockNotFound
		return
	}

	return
}

//
// In this file we're not checking for a few things, e.g.:
// - BaseFee
// - Signature validation
// - Amount == 0
//
// Those are part of `IsWellFormed` because they can be checked without context
// Not that when a fail condition is tested, the test is made to pass afterwards
// to ensure the error happened because of the expected cause, and not as a side
// effect of something else being broken
//
package runner

import (
	"testing"

	"boscoin.io/sebak/lib/block"
	"boscoin.io/sebak/lib/common"
	"boscoin.io/sebak/lib/error"
	"boscoin.io/sebak/lib/storage"
	"boscoin.io/sebak/lib/transaction"

	"github.com/stellar/go/keypair"
	"github.com/stretchr/testify/require"
)

// Test with some missing block accounts
func TestValidateTxPaymentMissingBlockAccount(t *testing.T) {
	kps, _ := keypair.Random()
	kpt, _ := keypair.Random()

	st := storage.NewTestStorage()
	defer st.Close()

	tx := transaction.Transaction{
		T: "transaction",
		H: transaction.TransactionHeader{
			Created: common.NowISO8601(),
		},
		B: transaction.TransactionBody{
			Source:     kps.Address(), // Need a well-formed address
			Fee:        common.BaseFee,
			SequenceID: 0,
			Operations: []transaction.Operation{
				transaction.Operation{
					H: transaction.OperationHeader{Type: transaction.OperationPayment},
					B: transaction.OperationBodyPayment{Target: kpt.Address(), Amount: common.Amount(10000)},
				},
			},
		},
	}
	tx.H.Hash = tx.B.MakeHashString()
	require.Equal(t, ValidateTx(st, tx), errors.ErrorBlockAccountDoesNotExists)

	// Now add the source account but not the target
	bas := block.BlockAccount{
		Address: kps.Address(),
		Balance: common.Amount(1 * common.AmountPerCoin),
	}
	bas.Save(st)
	require.Equal(t, ValidateTx(st, tx), errors.ErrorBlockAccountDoesNotExists)

	// Now just the target
	st1 := storage.NewTestStorage()
	defer st1.Close()
	bat := block.BlockAccount{
		Address: kpt.Address(),
		Balance: common.Amount(1 * common.AmountPerCoin),
	}
	bat.Save(st1)
	require.Equal(t, ValidateTx(st1, tx), errors.ErrorBlockAccountDoesNotExists)

	// And finally, bot
	st2 := storage.NewTestStorage()
	defer st2.Close()
	bas.Save(st2)
	bat.Save(st2)
	require.Nil(t, ValidateTx(st2, tx))
}

// Check for correct sequence ID
func TestValidateTxWrongSequenceID(t *testing.T) {
	kps, _ := keypair.Random()
	kpt, _ := keypair.Random()

	st := storage.NewTestStorage()
	defer st.Close()
	bas := block.BlockAccount{
		Address:    kps.Address(),
		Balance:    common.Amount(1 * common.AmountPerCoin),
		SequenceID: 1,
	}
	bat := block.BlockAccount{
		Address: kpt.Address(),
		Balance: common.Amount(1 * common.AmountPerCoin),
	}
	bas.Save(st)
	bat.Save(st)

	tx := transaction.Transaction{
		T: "transaction",
		H: transaction.TransactionHeader{
			Created: common.NowISO8601(),
		},
		B: transaction.TransactionBody{
			Source:     kps.Address(),
			Fee:        common.BaseFee,
			SequenceID: 0,
			Operations: []transaction.Operation{
				transaction.Operation{
					H: transaction.OperationHeader{Type: transaction.OperationPayment},
					B: transaction.OperationBodyPayment{Target: kpt.Address(), Amount: common.Amount(10000)},
				},
			},
		},
	}
	tx.H.Hash = tx.B.MakeHashString()
	require.Equal(t, ValidateTx(st, tx), errors.ErrorTransactionInvalidSequenceID)
	tx.B.SequenceID = 2
	require.Equal(t, ValidateTx(st, tx), errors.ErrorTransactionInvalidSequenceID)
	tx.B.SequenceID = 1
	require.Nil(t, ValidateTx(st, tx))
}

// Check sending the whole balance
func TestValidateTxOverBalance(t *testing.T) {
	kps, _ := keypair.Random()
	kpt, _ := keypair.Random()

	st := storage.NewTestStorage()
	defer st.Close()
	bas := block.BlockAccount{
		Address:    kps.Address(),
		Balance:    common.Amount(1 * common.AmountPerCoin),
		SequenceID: 1,
	}
	bat := block.BlockAccount{
		Address: kpt.Address(),
		Balance: common.Amount(1 * common.AmountPerCoin),
	}
	bas.Save(st)
	bat.Save(st)

	opbody := transaction.OperationBodyPayment{Target: kpt.Address(), Amount: bas.Balance}
	tx := transaction.Transaction{
		T: "transaction",
		H: transaction.TransactionHeader{
			Created: common.NowISO8601(),
		},
		B: transaction.TransactionBody{
			Source:     kps.Address(),
			Fee:        common.BaseFee,
			SequenceID: 1,
			Operations: []transaction.Operation{
				transaction.Operation{
					H: transaction.OperationHeader{Type: transaction.OperationPayment},
					B: opbody,
				},
			},
		},
	}
	tx.H.Hash = tx.B.MakeHashString()
	require.Equal(t, ValidateTx(st, tx), errors.ErrorTransactionExcessAbilityToPay)
	opbody.Amount = bas.Balance.MustSub(common.BaseFee)
	tx.B.Operations[0].B = opbody
	require.Nil(t, ValidateTx(st, tx))

	// Also test multiple operations
	// Note: The account balance is 1 BOS (10M units), so we make 4 ops of 2,5M
	// and check that the BaseFee are correctly calculated
	op := tx.B.Operations[0]
	opbody.Amount = common.Amount(2500000)
	op.B = opbody
	tx.B.Operations = []transaction.Operation{op, op, op, op}
	require.Equal(t, ValidateTx(st, tx), errors.ErrorTransactionExcessAbilityToPay)

	// Now the total amount of the ops + balance is equal to the balance
	opbody.Amount = opbody.Amount.MustSub(common.BaseFee.MustMult(len(tx.B.Operations)))
	tx.B.Operations[0].B = opbody
	require.Nil(t, ValidateTx(st, tx))
}

// Test creating an already existing account
func TestValidateOpCreateExistsAccount(t *testing.T) {
	kps, _ := keypair.Random()
	kpt, _ := keypair.Random()

	st := storage.NewTestStorage()
	defer st.Close()

	bas := block.BlockAccount{
		Address: kps.Address(),
		Balance: common.Amount(1 * common.AmountPerCoin),
	}
	bat := block.BlockAccount{
		Address: kpt.Address(),
		Balance: common.Amount(1 * common.AmountPerCoin),
	}
	bat.Save(st)
	bas.Save(st)

	tx := transaction.Transaction{
		T: "transaction",
		H: transaction.TransactionHeader{
			Created: common.NowISO8601(),
		},
		B: transaction.TransactionBody{
			Source:     kps.Address(), // Need a well-formed address
			Fee:        common.BaseFee,
			SequenceID: 0,
			Operations: []transaction.Operation{
				transaction.Operation{
					H: transaction.OperationHeader{Type: transaction.OperationCreateAccount},
					B: transaction.OperationBodyCreateAccount{Target: kpt.Address(), Amount: common.Amount(10000)},
				},
			},
		},
	}
	tx.H.Hash = tx.B.MakeHashString()
	require.Equal(t, ValidateTx(st, tx), errors.ErrorBlockAccountAlreadyExists)

	st1 := storage.NewTestStorage()
	defer st1.Close()
	bas.Save(st1)
	require.Nil(t, ValidateTx(st1, tx))
}

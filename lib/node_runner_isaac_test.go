package sebak

import (
	"context"
	"sync"
	"testing"

	"github.com/spikeekips/sebak/lib/network"
	"github.com/spikeekips/sebak/lib/util"
)

// TestNodeRunnerConsensus checks, all the validators can get consensus.
func TestNodeRunnerConsensus(t *testing.T) {
	defer network.CleanUpMemoryServer()

	numberOfNodes := 10
	nodeRunners := createNodeRunnersWithReady(numberOfNodes)
	for _, nr := range nodeRunners {
		defer nr.Stop()
	}

	var wg sync.WaitGroup

	wg.Add(numberOfNodes)

	var handleBallotCheckerFuncs = []util.CheckerFunc{
		CheckNodeRunnerHandleBallotIsWellformed,
		CheckNodeRunnerHandleBallotCheckIsNew,
		CheckNodeRunnerHandleBallotReceiveBallot,
		CheckNodeRunnerHandleBallotStore,
		CheckNodeRunnerHandleBallotBroadcast,
	}

	var dones []VotingStateStaging
	var deferFunc util.DeferFunc = func(n int, f util.CheckerFunc, ctx context.Context, err error) {
		if err == nil {
			return
		}

		if _, ok := err.(util.CheckerErrorStop); ok {
			vs, _ := ctx.Value("vs").(VotingStateStaging)
			if vs.State == BallotStateALLCONFIRM {
				dones = append(dones, vs)
				wg.Done()
			}
		}
	}

	ctx := context.WithValue(context.Background(), "deferFunc", deferFunc)
	for _, nr := range nodeRunners {
		nr.SetHandleBallotCheckerFuncs(ctx, handleBallotCheckerFuncs...)
	}

	nr0 := nodeRunners[0]

	client := nr0.TransportServer().GetClient(nr0.Node().Endpoint())

	tx := makeTransaction(nr0.Node().Keypair())
	client.SendMessage(tx)

	wg.Wait()

	for _, done := range dones {
		if done.State != BallotStateALLCONFIRM {
			t.Error("failed to get consensus")
			return
		}
		if done.MessageHash != tx.GetHash() {
			t.Error("failed to get consensus; found invalid message")
			return
		}
	}
}

// TestNodeRunnerConsensusWithVotingNO checks, consensus will be ended with
// VotingNO over threshold.
func TestNodeRunnerConsensusWithVotingNO(t *testing.T) {
	defer network.CleanUpMemoryServer()

	numberOfNodes := 3
	nodeRunners := createNodeRunnersWithReady(numberOfNodes)

	for _, nr := range nodeRunners {
		defer nr.Stop()
	}

	for _, nr := range nodeRunners {
		nr.Policy().Reset(BallotStateINIT, 100)
	}

	say_no_validators := []string{
		//nodeRunners[0].Node().Address(),
		nodeRunners[1].Node().Address(),
		nodeRunners[2].Node().Address(),
	}

	var wg sync.WaitGroup
	wg.Add(numberOfNodes)

	var handleBallotCheckerFuncs = []util.CheckerFunc{
		CheckNodeRunnerHandleBallotIsWellformed,
		CheckNodeRunnerHandleBallotCheckIsNew,
		CheckNodeRunnerHandleBallotReceiveBallot,
		// this will manipulate the VotingHole for `say_no_validators`
		func(ctx context.Context, target interface{}, args ...interface{}) (context.Context, error) {
			nr := target.(*NodeRunner)
			if !util.InStringArray(say_no_validators, nr.Node().Address()) {
				return ctx, nil
			}

			ballot := ctx.Value("ballot").(Ballot)
			if ballot.State() != BallotStateINIT {
				return ctx, nil
			}

			ballot.Vote(VotingNO)
			ballot.Sign(nr.Node().Keypair())

			ctx = context.WithValue(ctx, "ballot", ballot)

			return ctx, nil
		},

		CheckNodeRunnerHandleBallotStore,
		CheckNodeRunnerHandleBallotBroadcast,
	}

	var deferFunc util.DeferFunc = func(n int, f util.CheckerFunc, ctx context.Context, err error) {
		if err == nil {
			return
		}

		defer wg.Done()
		vs, _ := ctx.Value("vs").(VotingStateStaging)
		if !vs.IsClosed() {
			t.Error("VotingResult must be closed.")
			return
		}
		if vs.State != BallotStateINIT {
			t.Error("the final state must be `BallotStateINIT`.")
			return
		}
		if vs.VotingHole != VotingNO {
			t.Error("the final VotingHole must be `VotingNO`.")
			return
		}
	}

	ctx := context.WithValue(context.Background(), "deferFunc", deferFunc)
	for _, nr := range nodeRunners {
		nr.SetHandleBallotCheckerFuncs(ctx, handleBallotCheckerFuncs...)
	}
	nr0 := nodeRunners[0]

	client := nr0.TransportServer().GetClient(nr0.Node().Endpoint())

	tx := makeTransaction(nr0.Node().Keypair())
	client.SendMessage(tx)

	wg.Wait()
}
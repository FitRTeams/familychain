package property

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"https://github.com/FitRTeams/familychain/e2e-polybft/framework"
	"https://github.com/FitRTeams/familychain/types"
	"pgregory.net/rapid"
)

func TestProperty_DifferentVotingPower(t *testing.T) {
	t.Parallel()

	const (
		blockTime  = time.Second * 6
		maxPremine = math.MaxUint64
	)

	rapid.Check(t, func(tt *rapid.T) {
		var (
			numNodes  = rapid.Uint64Range(4, 8).Draw(tt, "number of cluster nodes")
			epochSize = rapid.OneOf(rapid.Just(4), rapid.Just(10)).Draw(tt, "epoch size")
			numBlocks = rapid.Uint64Range(2, 5).Draw(tt, "number of blocks the cluster should mine")
		)

		premine := make([]uint64, numNodes)

		// premined amount will determine validator's stake and therefore voting power
		for i := range premine {
			premine[i] = rapid.Uint64Range(1, maxPremine).Draw(tt, fmt.Sprintf("stake for node %d", i+1))
		}

		cluster := framework.NewTestCluster(t, int(numNodes),
			framework.WithEpochSize(epochSize),
			framework.WithSecretsCallback(func(adresses []types.Address, config *framework.TestClusterConfig) {
				for i, a := range adresses {
					config.PremineValidators = append(config.PremineValidators, fmt.Sprintf("%s:%d", a, premine[i]))
				}
			}))
		defer cluster.Stop()

		// wait for single epoch to process withdrawal
		require.NoError(t, cluster.WaitForBlock(numBlocks, blockTime*time.Duration(numBlocks)))
	})
}

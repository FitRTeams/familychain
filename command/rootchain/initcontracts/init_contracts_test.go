package initcontracts

import (
	"os"
	"testing"

	"github.com/FamilyChain/family/command"
	"github.com/FamilyChain/family/command/rootchain/helper"
	"github.com/FamilyChain/family/consensus/polybft"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo/jsonrpc"
	"github.com/umbracle/ethgo/testutil"
)

func TestDeployContracts_NoPanics(t *testing.T) {
	t.Parallel()

	server := testutil.DeployTestServer(t, nil)
	t.Cleanup(func() {
		err := os.RemoveAll(params.manifestPath)
		if err != nil {
			t.Fatal(err)
		}
	})

	client, err := jsonrpc.NewClient(server.HTTPAddr())
	require.NoError(t, err)

	testKey, err := helper.GetRootchainPrivateKey("")
	require.NoError(t, err)

	receipt, err := server.Fund(testKey.Address())
	require.NoError(t, err)
	require.Equal(t, uint64(1), receipt.Status)

	outputter := command.InitializeOutputter(GetCommand())

	require.NotPanics(t, func() {
		err = deployContracts(outputter, client, &polybft.Manifest{})
	})
	require.NoError(t, err)
}

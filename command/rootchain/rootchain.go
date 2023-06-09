package rootchain

import (
	"github.com/spf13/cobra"

	"github.com/FitRTeams/familychain/command/rootchain/fund"
	"github.com/FitRTeams/familychain/command/rootchain/initcontracts"
	"github.com/FitRTeams/familychain/command/rootchain/server"
)

// GetCommand creates "rootchain" helper command
func GetCommand() *cobra.Command {
	rootchainCmd := &cobra.Command{
		Use:   "rootchain",
		Short: "Top level rootchain helper command.",
	}

	registerSubcommands(rootchainCmd)

	return rootchainCmd
}

func registerSubcommands(baseCmd *cobra.Command) {
	baseCmd.AddCommand(
		// rootchain fund
		fund.GetCommand(),
		// rootchain server
		server.GetCommand(),
		// init-contracts
		initcontracts.GetCommand(),
	)
}

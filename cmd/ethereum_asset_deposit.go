package cmd

import (
	"context"
	"fmt"
	"math/big"

	vgethereum "code.vegaprotocol.io/shared/libs/ethereum"
	"code.vegaprotocol.io/vegacapsule/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
)

type ethereumAssetDepositOrStakeArgs struct {
	amount          int64
	vegaPubKey      string
	ownerPrivateKey string
	bridgeAddress   string
	assetAddress    string
	networkAddress  string
}

var (
	ethereumAssetDepositFlags = struct {
		vegaPubKey  string
		assetSymbol string
		amount      int64
	}{}
)

func init() {
	ethereumAssetDepositCmd.Flags().StringVar(&ethereumAssetDepositFlags.assetSymbol, "asset-symbol", "", "symbol of the asset to be deposited")
	ethereumAssetDepositCmd.Flags().StringVar(&ethereumAssetDepositFlags.vegaPubKey, "pub-key", "", "Vega public key to where the asset will be deposited")
	ethereumAssetDepositCmd.Flags().Int64Var(&ethereumAssetDepositFlags.amount, "amount", 0, "amount to be deposited")
	ethereumAssetDepositCmd.MarkFlagRequired("asset-symbol")
	ethereumAssetDepositCmd.MarkFlagRequired("pub-key")
	ethereumAssetDepositCmd.MarkFlagRequired("amount")
}

var ethereumAssetDepositCmd = &cobra.Command{
	Use:   "deposit",
	Short: "Deposit allows to deposit an asset to given Vega public key.",
	RunE: func(cmd *cobra.Command, args []string) error {
		netState, err := state.LoadNetworkState(homePath)
		if err != nil {
			return err
		}

		if netState.Empty() {
			return networkNotBootstrappedErr("ethereum asset deposit")
		}

		if !netState.Running() {
			return networkNotRunningErr("ethereum asset deposit")
		}

		conf := netState.Config

		smartContracts, err := conf.SmartContractsInfo()
		if err != nil {
			return fmt.Errorf("failed getting smart contract informations: %w", err)
		}

		asset := conf.GetSmartContractToken(ethereumAssetDepositFlags.assetSymbol)
		if asset == nil {
			return fmt.Errorf("failed to get non existing asset: %q", ethereumAssetDepositFlags.assetSymbol)
		}

		depositArgs := ethereumAssetDepositOrStakeArgs{
			amount:          ethereumAssetDepositFlags.amount,
			vegaPubKey:      ethereumAssetDepositFlags.vegaPubKey,
			ownerPrivateKey: smartContracts.EthereumOwner.Private,
			bridgeAddress:   smartContracts.ERC20Bridge.EthereumAddress,
			assetAddress:    asset.EthereumAddress,
			networkAddress:  conf.Network.Ethereum.Endpoint,
		}

		return ethereumAssetDeposit(cmd.Context(), depositArgs)
	},
}

func ethereumAssetDeposit(ctx context.Context, args ethereumAssetDepositOrStakeArgs) error {
	client, err := vgethereum.NewClient(ctx, args.networkAddress)
	if err != nil {
		return fmt.Errorf("failed to create Ethereum client: %w", err)
	}

	syncTimeout := defeaultSyncTimeout()
	bridgeAddr := common.HexToAddress(args.bridgeAddress)

	bridgeSession, err := client.NewERC20BridgeSession(
		ctx,
		args.ownerPrivateKey,
		bridgeAddr,
		&syncTimeout,
	)
	if err != nil {
		return fmt.Errorf("failed to create erc20 bridge session for %s: %w", args.bridgeAddress, err)
	}

	tokenSession, err := client.NewBaseTokenSession(
		ctx,
		args.ownerPrivateKey,
		common.HexToAddress(args.assetAddress),
		&syncTimeout,
	)
	if err != nil {
		return fmt.Errorf("failed to create token session for %s: %w", args.assetAddress, err)
	}

	amount := big.NewInt(args.amount)

	if _, err := tokenSession.ApproveSync(bridgeAddr, amount); err != nil {
		return fmt.Errorf("failed to approve asset amount to bridge: %w", err)
	}

	vegaPubKeyArr, err := vgethereum.HexStringToByte32Array(args.vegaPubKey)
	if err != nil {
		return fmt.Errorf("failed to convert Vega pub key string to byte array: %w", err)
	}

	tx, err := bridgeSession.DepositAssetSync(
		common.HexToAddress(args.assetAddress),
		amount,
		vegaPubKeyArr,
	)
	if err != nil {
		return fmt.Errorf("failed to deposit asset: %w", err)
	}

	return printEthereumTx(tx)
}

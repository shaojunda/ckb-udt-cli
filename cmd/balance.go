package cmd

import (
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/address"
	"github.com/nervosnetwork/ckb-sdk-go/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/ququzone/ckb-udt-cli/config"
	"github.com/spf13/cobra"
	"math/big"
)

var (
	balanceConf *string
	balanceUUID *string
	balanceAddr *string
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Query sUDT balance",
	Long:  `Query sUDT balance by address.`,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := config.Init(*balanceConf)
		if err != nil {
			Fatalf("load config error: %v", err)
		}

		client, err := rpc.DialWithIndexer(c.RPC, c.CkbIndexer)
		if err != nil {
			Fatalf("create rpc client error: %v", err)
		}

		addr, err := address.Parse(*balanceAddr)
		if err != nil {
			Fatalf("parse address error: %v", err)
		}
		searchKey := &indexer.SearchKey{
			Script:     addr.Script,
			ScriptType: "lock",
		}
		cells, err := CollectUDT(client, c, searchKey, "asc", 1000, "", types.HexToHash(*balanceUUID).Bytes(), nil)
		if err != nil {
			Fatalf("collect cell error: %v", err)
		}

		fmt.Printf("Address %s amount: %s\n", *balanceAddr, cells.Options["total"].(*big.Int).String())
	},
}

func init() {
	rootCmd.AddCommand(balanceCmd)

	balanceConf = balanceCmd.Flags().StringP("config", "c", "config.yaml", "Config file")
	balanceUUID = balanceCmd.Flags().StringP("uuid", "u", "", "UDT uuid")
	balanceAddr = balanceCmd.Flags().StringP("address", "a", "", "Address")
	_ = balanceCmd.MarkFlagRequired("uuid")
	_ = balanceCmd.MarkFlagRequired("address")
}

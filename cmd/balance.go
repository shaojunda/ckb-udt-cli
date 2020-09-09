package cmd

import (
	"fmt"
	"math/big"
	"strconv"

	"github.com/nervosnetwork/ckb-sdk-go/address"
	"github.com/nervosnetwork/ckb-sdk-go/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/ququzone/ckb-udt-cli/config"
	"github.com/spf13/cobra"
)

var (
	balanceConf            *string
	balanceUUID            *string
	balanceAddr            *string
	balanceFromBlockNumber *string
)

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Query sUDT balance",
	Long:  `Query sUDT balance by address.`,
	Run: func(cmd *cobra.Command, args []string) {
		var unitFromBlockNumber uint64
		var err error
		if *balanceFromBlockNumber == "" {
			unitFromBlockNumber = 0
		} else {
			unitFromBlockNumber, err = strconv.ParseUint(*balanceFromBlockNumber, 10, 64)
			if err != nil {
				Fatalf("fromBlockNumber invalid: %v", err)
			}
		}
		c, err := config.Init(*balanceConf)
		if err != nil {
			Fatalf("load config error: %v", err)
		}

		client, err := rpc.Dial(c.RPC)
		if err != nil {
			Fatalf("create rpc client error: %v", err)
		}

		addr, err := address.Parse(*balanceAddr)
		if err != nil {
			Fatalf("parse address error: %v", err)
		}

		cells, err := CollectUDT(client, c, addr.Script, types.HexToHash(*balanceUUID).Bytes(), nil, unitFromBlockNumber)
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
	balanceFromBlockNumber = balanceCmd.Flags().StringP("balanceFromBlockNumber", "f", "", "From block number")
	_ = balanceCmd.MarkFlagRequired("uuid")
	_ = balanceCmd.MarkFlagRequired("address")
}

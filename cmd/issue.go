package cmd

import (
	"context"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/crypto/secp256k1"
	"github.com/nervosnetwork/ckb-sdk-go/indexer"
	"github.com/nervosnetwork/ckb-sdk-go/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/transaction"
	"github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/nervosnetwork/ckb-sdk-go/utils"
	"github.com/ququzone/ckb-udt-cli/config"
	"github.com/spf13/cobra"
	"math/big"
)

var (
	issueConf   *string
	issueKey    *string
	issueAmount *string
)

var issueCmd = &cobra.Command{
	Use:   "issue",
	Short: "Issue sUDT token",
	Long:  `Issue sUDT with secp256k1 cell.`,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := config.Init(*issueConf)
		if err != nil {
			Fatalf("load config error: %v", err)
		}

		client, err := rpc.DialWithIndexer(c.RPC, c.CkbIndexer)
		if err != nil {
			Fatalf("create rpc client error: %v", err)
		}

		key, err := secp256k1.HexToKey(*issueKey)
		if err != nil {
			Fatalf("import private key error: %v", err)
		}

		scripts, err := utils.NewSystemScripts(client)
		if err != nil {
			Fatalf("load system script error: %v", err)
		}

		change, err := key.Script(scripts)

		capacity := uint64(14200000000)
		fee := uint64(1000)
		searchKey := &indexer.SearchKey{
			Script:     change,
			ScriptType: "lock",
		}
		cellCollector := utils.NewLiveCellCollector(client, searchKey, "asc", 1000, "", utils.NewCapacityLiveCellProcessor(capacity+fee))
		cells, err := cellCollector.Collect()
		if err != nil {
			Fatalf("collect cell error: %v", err)
		}

		if cells.Capacity < capacity+fee {
			Fatalf("insufficient capacity: %d < %d", cells.Capacity, capacity+fee)
		}

		tx := transaction.NewSecp256k1SingleSigTx(scripts)
		for _, dep := range c.UDT.Deps {
			tx.CellDeps = append(tx.CellDeps, &types.CellDep{
				OutPoint: &types.OutPoint{
					TxHash: types.HexToHash(dep.TxHash),
					Index:  dep.Index,
				},
				DepType: types.DepType(dep.DepType),
			})
		}
		uuid, _ := change.Hash()

		tx.Outputs = append(tx.Outputs, &types.CellOutput{
			Capacity: uint64(capacity),
			Lock: &types.Script{
				CodeHash: change.CodeHash,
				HashType: change.HashType,
				Args:     change.Args,
			},
			Type: &types.Script{
				CodeHash: types.HexToHash(c.UDT.Script.CodeHash),
				HashType: types.ScriptHashType(c.UDT.Script.HashType),
				Args:     uuid.Bytes(),
			},
		})
		a, _ := big.NewInt(0).SetString(*issueAmount, 10)
		b := a.Bytes()
		for i := 0; i < len(b)/2; i++ {
			b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
		}
		if len(b) < 16 {
			for i := len(b); i < 16; i++ {
				b = append(b, 0)
			}
		}
		tx.OutputsData = append(tx.OutputsData, b)
		if cells.Capacity-capacity+fee > 6100000000 {
			tx.Outputs = append(tx.Outputs, &types.CellOutput{
				Capacity: cells.Capacity - capacity - fee,
				Lock:     change,
			})
			tx.OutputsData = append(tx.OutputsData, []byte{})
		} else {
			tx.Outputs[0].Capacity = tx.Outputs[0].Capacity + cells.Capacity - capacity - fee
		}
		var inputs []*types.CellInput
		for _, cell := range cells.LiveCells {
			inputs = append(inputs, &types.CellInput{
				Since:          0,
				PreviousOutput: cell.OutPoint,
			})
		}
		group, witnessArgs, err := transaction.AddInputsForTransaction(tx, inputs)
		if err != nil {
			Fatalf("add inputs to transaction error: %v", err)
		}

		err = transaction.SingleSignTransaction(tx, group, witnessArgs, key)
		if err != nil {
			Fatalf("sign transaction error: %v", err)
		}

		hash, err := client.SendTransaction(context.Background(), tx)
		if err != nil {
			Fatalf("send transaction error: %v", err)
		}

		fmt.Printf("Issued sUDT transaction hash: %s, uuid: %s\n", hash.String(), uuid.String())
	},
}

func init() {
	rootCmd.AddCommand(issueCmd)

	issueConf = issueCmd.Flags().StringP("config", "c", "config.yaml", "Config file")
	issueKey = issueCmd.Flags().StringP("key", "k", "", "Issue private key")
	issueAmount = issueCmd.Flags().StringP("amount", "a", "", "Issue amount")
	_ = issueCmd.MarkFlagRequired("key")
	_ = issueCmd.MarkFlagRequired("amount")
}

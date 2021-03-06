package cmd

import (
	"context"
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/address"
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
	transferConf   *string
	transferKey    *string
	transferAmount *string
	transferTo     *string
	transferUUID   *string
)

var transferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer sUDT token",
	Long:  `Transfer sUDT from secp256k1 lock cell.`,
	Run: func(cmd *cobra.Command, args []string) {
		c, err := config.Init(*transferConf)
		if err != nil {
			Fatalf("load config error: %v", err)
		}

		client, err := rpc.DialWithIndexer(c.RPC, c.CkbIndexer)
		if err != nil {
			Fatalf("create rpc client error: %v", err)
		}

		key, err := secp256k1.HexToKey(*transferKey)
		if err != nil {
			Fatalf("import private key error: %v", err)
		}

		amount := big.NewInt(0)
		amount, _ = amount.SetString(*transferAmount, 10)
		if amount == nil || amount.Uint64() == 0 {
			Fatalf("transfer amount error: %s", *transferAmount)
		}

		uuid := types.HexToHash(*transferUUID).Bytes()

		capacity := uint64(28400000000)
		fee := uint64(953)
		recipientAddr, err := address.Parse(*transferTo)
		if err != nil {
			Fatalf("parse to address error: %v", err)
		}

		var recipientCell *indexer.LiveCell
		if recipientAddr.Script.CodeHash.String() == c.ACP.Script.CodeHash {
			searchKey := &indexer.SearchKey{
				Script:     recipientAddr.Script,
				ScriptType: "lock",
			}
			cells, err := CollectUDT(client, c, searchKey, "asc", 1000, "", uuid, big.NewInt(0))
			if err != nil {
				Fatalf("collect cell error: %v", err)
			}
			if len(cells.LiveCells) == 0 {
				Fatalf("can't find anyone can pay cell for %s", *transferTo)
			}
			recipientCell = cells.LiveCells[0]
		}

		scripts, err := utils.NewSystemScripts(client)
		if err != nil {
			Fatalf("load system script error: %v", err)
		}

		fromAcp := true
		fromSecp256k1Script, err := key.Script(scripts)
		if err != nil {
			Fatalf("load system script error: %v", err)
		}
		fromScript := &types.Script{
			CodeHash: types.HexToHash(c.ACP.Script.CodeHash),
			HashType: types.ScriptHashType(c.ACP.Script.HashType),
			Args:     fromSecp256k1Script.Args,
		}
		searchKey := &indexer.SearchKey{
			Script:     fromScript,
			ScriptType: "lock",
		}
		cells, err := CollectUDT(client, c, searchKey, "asc", 1000, "", uuid, amount)
		if err != nil {
			Fatalf("collect cell error: %v", err)
		}

		if cells.Options["total"].(*big.Int).Cmp(amount) < 0 {
			fromScript = fromSecp256k1Script
			searchKey := &indexer.SearchKey{
				Script:     fromScript,
				ScriptType: "lock",
			}
			cells, err = CollectUDT(client, c, searchKey, "asc", 1000, "", uuid, amount)
			if err != nil {
				Fatalf("collect cell error: %v", err)
			}
			if cells.Options["total"].(*big.Int).Cmp(amount) < 0 {
				Fatalf("insufficient UDT balance")
			}
			fromAcp = false
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
		if fromAcp || recipientCell != nil {
			for _, dep := range c.ACP.Deps {
				tx.CellDeps = append(tx.CellDeps, &types.CellDep{
					OutPoint: &types.OutPoint{
						TxHash: types.HexToHash(dep.TxHash),
						Index:  dep.Index,
					},
					DepType: types.DepType(dep.DepType),
				})
			}
		}

		var feeCells *utils.LiveCellCollectResult
		searchKey = &indexer.SearchKey{
			Script:     fromScript,
			ScriptType: "lock",
		}
		if cells.Capacity < capacity+fee {
			cellCollector := utils.NewLiveCellCollector(client, searchKey, "asc", 1000, "", utils.NewCapacityLiveCellProcessor(capacity+fee-cells.Capacity))
			cellCollector.EmptyData = true
			feeCells, err = cellCollector.Collect()
			if err != nil {
				Fatalf("collect cell error: %v", err)
			}

			if feeCells.Capacity < capacity+fee-cells.Capacity {
				Fatalf("insufficient capacity: %d < %d", cells.Capacity+feeCells.Capacity, capacity+fee)
			}
		}

		if recipientCell != nil {
			capacity -= 14200000000
			input := &types.CellInput{
				Since: 0,
				PreviousOutput: &types.OutPoint{
					TxHash: recipientCell.OutPoint.TxHash,
					Index:  recipientCell.OutPoint.Index,
				},
			}
			tx.Inputs = append(tx.Inputs, input)
			tx.Witnesses = append(tx.Witnesses, []byte{})
			tx.Outputs = append(tx.Outputs, &types.CellOutput{
				Capacity: recipientCell.Output.Capacity,
				Lock:     recipientCell.Output.Lock,
				Type:     recipientCell.Output.Type,
			})
			originTx, err := client.GetTransaction(context.Background(), recipientCell.OutPoint.TxHash)
			if err != nil {
				Fatalf("query anyone can pay transaction error: %v", err)
			}
			b := originTx.Transaction.OutputsData[recipientCell.OutPoint.Index]
			for i := 0; i < len(b)/2; i++ {
				b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
			}
			origin := big.NewInt(0).SetBytes(b)

			b = big.NewInt(0).Add(origin, amount).Bytes()
			for i := 0; i < len(b)/2; i++ {
				b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
			}
			if len(b) < 16 {
				for i := len(b); i < 16; i++ {
					b = append(b, 0)
				}
			}
			tx.OutputsData = append(tx.OutputsData, b)
		} else {
			tx.Outputs = append(tx.Outputs, &types.CellOutput{
				Capacity: 14200000000,
				Lock:     recipientAddr.Script,
				Type: &types.Script{
					CodeHash: types.HexToHash(c.UDT.Script.CodeHash),
					HashType: types.ScriptHashType(c.UDT.Script.HashType),
					Args:     uuid,
				},
			})

			b := amount.Bytes()
			for i := 0; i < len(b)/2; i++ {
				b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
			}
			if len(b) < 16 {
				for i := len(b); i < 16; i++ {
					b = append(b, 0)
				}
			}
			tx.OutputsData = append(tx.OutputsData, b)
		}

		tx.Outputs = append(tx.Outputs, &types.CellOutput{
			Capacity: 14200000000,
			Lock:     fromScript,
			Type: &types.Script{
				CodeHash: types.HexToHash(c.UDT.Script.CodeHash),
				HashType: types.ScriptHashType(c.UDT.Script.HashType),
				Args:     uuid,
			},
		})
		if cells.Options["total"].(*big.Int).Cmp(amount) == 0 {
			tx.OutputsData = append(tx.OutputsData, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
		} else {
			b := big.NewInt(0).Sub(cells.Options["total"].(*big.Int), amount).Bytes()
			for i := 0; i < len(b)/2; i++ {
				b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
			}
			if len(b) < 16 {
				for i := len(b); i < 16; i++ {
					b = append(b, 0)
				}
			}
			tx.OutputsData = append(tx.OutputsData, b)
		}

		if cells.Capacity-capacity-fee >= 6100000000 || (feeCells != nil && cells.Capacity-capacity-fee+feeCells.Capacity >= 6100000000) {
			change := cells.Capacity - fee - 14200000000
			if feeCells != nil {
				change += feeCells.Capacity
			}
			if recipientCell == nil {
				change -= 14200000000
			}
			if change >= 6100000000 {
				tx.Outputs = append(tx.Outputs, &types.CellOutput{
					Capacity: change,
					Lock:     fromScript,
				})

				tx.OutputsData = append(tx.OutputsData, []byte{})
			}
		} else {
			change := cells.Capacity - fee
			if feeCells != nil {
				change += feeCells.Capacity
			}
			if recipientCell == nil {
				change -= 14200000000
			}
			tx.Outputs[1].Capacity = change
		}

		var inputs []*types.CellInput
		for _, cell := range cells.LiveCells {
			inputs = append(inputs, &types.CellInput{
				Since:          0,
				PreviousOutput: cell.OutPoint,
			})
		}
		if feeCells != nil {
			for _, cell := range feeCells.LiveCells {
				inputs = append(inputs, &types.CellInput{
					Since:          0,
					PreviousOutput: cell.OutPoint,
				})
			}
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
			fmt.Println(rpc.TransactionString(tx))
			Fatalf("send transaction error: %v", err)
		}

		fmt.Printf("transfer transaction hash: %s\n", hash.String())
	},
}

func init() {
	rootCmd.AddCommand(transferCmd)

	transferConf = transferCmd.Flags().StringP("config", "c", "config.yaml", "Config file")
	transferKey = transferCmd.Flags().StringP("key", "k", "", "From private key")
	transferUUID = transferCmd.Flags().StringP("uuid", "u", "", "UDT uuid")
	transferAmount = transferCmd.Flags().StringP("amount", "a", "", "Transfer amount")
	transferTo = transferCmd.Flags().StringP("to", "t", "", "Transfer recipient address")
	_ = transferCmd.MarkFlagRequired("key")
	_ = transferCmd.MarkFlagRequired("amount")
	_ = transferCmd.MarkFlagRequired("uuid")
	_ = transferCmd.MarkFlagRequired("to")
}

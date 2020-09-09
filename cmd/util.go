package cmd

import (
	"context"
	"fmt"
	"github.com/ququzone/ckb-udt-cli/config"
	"math/big"
	"os"

	"github.com/nervosnetwork/ckb-sdk-go/rpc"
	"github.com/nervosnetwork/ckb-sdk-go/types"
	"github.com/nervosnetwork/ckb-sdk-go/utils"
)

func Fatalf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
	os.Exit(1)
}

type UDTCellProcessor struct {
	Client rpc.Client
	Max    *big.Int
}

func NewUDTCellProcessor(client rpc.Client, max *big.Int) *UDTCellProcessor {
	return &UDTCellProcessor{
		Client: client,
		Max:    max,
	}
}

func (p *UDTCellProcessor) Process(cell *types.Cell, result *utils.CollectResult) (bool, error) {
	result.Capacity = result.Capacity + cell.Capacity
	result.Cells = append(result.Cells, cell)

	tx, err := p.Client.GetTransaction(context.Background(), cell.OutPoint.TxHash)
	if err != nil {
		return false, err
	}
	b := tx.Transaction.OutputsData[cell.OutPoint.Index]
	for i := 0; i < len(b)/2; i++ {
		b[i], b[len(b)-i-1] = b[len(b)-i-1], b[i]
	}
	amount := big.NewInt(0).SetBytes(b)
	total, ok := result.Options["total"]
	if ok {
		result.Options["total"] = big.NewInt(0).Add(total.(*big.Int), amount)
	} else {
		result.Options["total"] = amount
	}
	if p.Max != nil && result.Options["total"].(*big.Int).Cmp(p.Max) >= 0 {
		return true, nil
	}
	return false, nil
}

func CollectUDT(client rpc.Client, c *config.Config, lock *types.Script, uuid []byte, max *big.Int, fromBlockNumber uint64) (*utils.CollectResult, error) {
	cellCollector := utils.NewCellCollector(client, lock, NewUDTCellProcessor(client, max), fromBlockNumber)
	cellCollector.EmptyData = false
	cellCollector.TypeScript = &types.Script{
		CodeHash: types.HexToHash(c.UDT.Script.CodeHash),
		HashType: types.ScriptHashType(c.UDT.Script.HashType),
		Args:     uuid,
	}
	cells, err := cellCollector.Collect()
	if err != nil {
		return nil, err
	}
	if _, ok := cells.Options["total"]; !ok {
		cells.Options["total"] = big.NewInt(0)
	}
	return cells, nil
}

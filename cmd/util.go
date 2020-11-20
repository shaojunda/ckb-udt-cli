package cmd

import (
	"fmt"
	"github.com/nervosnetwork/ckb-sdk-go/indexer"
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

func (p *UDTCellProcessor) Process(liveCell *indexer.LiveCell, result *utils.LiveCellCollectResult) (bool, error) {
	result.Capacity = result.Capacity + liveCell.Output.Capacity
	result.LiveCells = append(result.LiveCells, liveCell)
	amount, err := utils.ParseSudtAmount(liveCell.OutputData)
	if err != nil {
		return false, err
	}
	total, ok := result.Options["total"]
	if ok {
		result.Options["total"] = big.NewInt(0).Add(total.(*big.Int), amount)
	} else {
		result.Options = make(map[string]interface{})
		result.Options["total"] = amount
	}
	if p.Max != nil && result.Options["total"].(*big.Int).Cmp(p.Max) >= 0 {
		return true, nil
	}
	return false, nil
}

func CollectUDT(client rpc.Client, c *config.Config, searchKey *indexer.SearchKey, searchOrder indexer.SearchOrder, limit uint64, afterCursor string, uuid []byte, max *big.Int) (*utils.LiveCellCollectResult, error) {
	cellCollector := utils.NewLiveCellCollector(client, searchKey, searchOrder, limit, afterCursor, NewUDTCellProcessor(client, max))
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
	if cells.Options == nil {
		cells.Options = make(map[string]interface{})
	}
	if _, ok := cells.Options["total"]; !ok {
		cells.Options["total"] = big.NewInt(0)
	}
	return cells, nil
}

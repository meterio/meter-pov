package debug

import (
	"fmt"

	"github.com/dfinlab/meter/meter"

	"github.com/dfinlab/meter/vm"
	"github.com/ethereum/go-ethereum/common/math"
)

type TracerOption struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

type ExecutionResult struct {
	Gas         uint64         `json:"gas"`
	Failed      bool           `json:"failed"`
	ReturnValue string         `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}

type StructLogRes struct {
	Pc      uint64             `json:"pc"`
	Op      string             `json:"op"`
	Gas     uint64             `json:"gas"`
	GasCost uint64             `json:"gasCost"`
	Depth   int                `json:"depth"`
	Error   error              `json:"error,omitempty"`
	Stack   *[]string          `json:"stack,omitempty"`
	Memory  *[]string          `json:"memory,omitempty"`
	Storage *map[string]string `json:"storage,omitempty"`
}

type TraceFilterOptions struct {
	FromBlock   string          `json:"fromBlock"`
	ToBlock     string          `json:"toBlock"`
	FromAddress []meter.Address `json:"fromAddress"`
	ToAddress   []meter.Address `json:"toAddress"`
	After       uint64          `json:"after"`
	Count       uint64          `json:"count"`
}

type TraceAction struct {
	CallType string        `json:"callType"`
	From     meter.Address `json:"from"`
	Gas      string        `json:"gas"`
	Input    string        `json:"input"`
	To       meter.Address `json:"to"`
	Value    string        `json:"value"`
}
type TraceDataResult struct {
	GasUsed string `json:"gasUsed"`
	Output  string `json:"output"`
}
type TraceData struct {
	Action              TraceAction     `json:"action"`
	BlockHash           meter.Bytes32   `json:"blockHash"`
	BlockNumber         uint64          `json:"blockNumber"`
	Result              TraceDataResult `json:"result"`
	Subtraces           uint64          `json:"subtraces"`
	TraceAddress        []uint64        `json:"traceAddress"`
	TransactionHash     meter.Bytes32   `json:"transactionHash"`
	TransactionPosition uint64          `json:"transactionPosition"`
	Type                string          `json:"type"`
}

type CallTraceResult struct {
	Type    string            `json:"type"`
	From    string            `json:"from"`
	To      string            `json:"to"`
	Value   string            `json:"value"`
	Gas     string            `json:"gas"`
	GasUsed string            `json:"gasUsed"`
	Input   string            `json:"input"`
	Output  string            `json:"output"`
	Calls   []CallTraceResult `json:"calls"`
}

// formatLogs formats EVM returned structured logs for json output
func formatLogs(logs []vm.StructLog) []StructLogRes {
	formatted := make([]StructLogRes, len(logs))
	for index, trace := range logs {
		formatted[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.Err,
		}
		if trace.Stack != nil {
			stack := make([]string, len(trace.Stack))
			for i, stackValue := range trace.Stack {
				stack[i] = fmt.Sprintf("%x", math.PaddedBigBytes(stackValue, 32))
			}
			formatted[index].Stack = &stack
		}
		if trace.Memory != nil {
			memory := make([]string, 0, (len(trace.Memory)+31)/32)
			for i := 0; i+32 <= len(trace.Memory); i += 32 {
				memory = append(memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
			}
			formatted[index].Memory = &memory
		}
		if trace.Storage != nil {
			storage := make(map[string]string)
			for i, storageValue := range trace.Storage {
				storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
			}
			formatted[index].Storage = &storage
		}
	}
	return formatted
}

type StorageRangeOption struct {
	Address   meter.Address
	KeyStart  string
	MaxResult int
	Target    string
}

type StorageRangeResult struct {
	Storage StorageMap     `json:"storage"`
	NextKey *meter.Bytes32 `json:"nextKey"` // nil if Storage includes the last key in the trie.
}

type StorageMap map[string]StorageEntry

type StorageEntry struct {
	Key   *meter.Bytes32 `json:"key"`
	Value *meter.Bytes32 `json:"value"`
}

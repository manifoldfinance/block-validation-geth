package blockvalidation

import (
	"encoding/json"
	"errors"
	"fmt"

	capellaapi "github.com/attestantio/go-builder-client/api/capella"
	"github.com/attestantio/go-eth2-client/spec/phase0"
	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"

	boostTypes "github.com/flashbots/go-boost-utils/types"
)

func verifyWithdrawals(withdrawals types.Withdrawals, expectedWithdrawalsRoot common.Hash, isShanghai bool) error {
	if !isShanghai {
		// Reject payload attributes with withdrawals before shanghai
		if withdrawals != nil {
			return errors.New("withdrawals before shanghai")
		}
		return nil
	}

	withdrawalsRoot := types.DeriveSha(types.Withdrawals(withdrawals), trie.NewStackTrie(nil))
	if withdrawalsRoot != expectedWithdrawalsRoot {
		return fmt.Errorf("withdrawals mismatch")
	}
	return nil
}

// Register adds catalyst APIs to the full node.
func Register(stack *node.Node, backend *eth.Ethereum) error {

	stack.RegisterAPIs([]rpc.API{
		{
			Namespace: "flashbots",
			Service:   NewBlockValidationAPI(backend),
		},
	})
	return nil
}

type BlockValidationAPI struct {
	eth *eth.Ethereum
}

// NewConsensusAPI creates a new consensus api for the given backend.
// The underlying blockchain needs to have a valid terminal total difficulty set.
func NewBlockValidationAPI(eth *eth.Ethereum) *BlockValidationAPI {
	return &BlockValidationAPI{
		eth: eth,
	}
}

type BuilderBlockValidationRequest struct {
	boostTypes.BuilderSubmitBlockRequest
	RegisteredGasLimit uint64 `json:"registered_gas_limit,string"`
}

func (api *BlockValidationAPI) ValidateBuilderSubmissionV1(params *BuilderBlockValidationRequest) error {
	// TODO: fuzztest, make sure the validation is sound
	// TODO: handle context!

	if params.ExecutionPayload == nil {
		return errors.New("nil execution payload")
	}
	payload := params.ExecutionPayload
	block, err := engine.ExecutionPayloadToBlock(payload)
	if err != nil {
		return err
	}

	if params.Message.ParentHash != boostTypes.Hash(block.ParentHash()) {
		return fmt.Errorf("incorrect ParentHash %s, expected %s", params.Message.ParentHash.String(), block.ParentHash().String())
	}

	if params.Message.BlockHash != boostTypes.Hash(block.Hash()) {
		return fmt.Errorf("incorrect BlockHash %s, expected %s", params.Message.BlockHash.String(), block.Hash().String())
	}

	if params.Message.GasLimit != block.GasLimit() {
		return fmt.Errorf("incorrect GasLimit %d, expected %d", params.Message.GasLimit, block.GasLimit())
	}

	if params.Message.GasUsed != block.GasUsed() {
		return fmt.Errorf("incorrect GasUsed %d, expected %d", params.Message.GasUsed, block.GasUsed())
	}

	feeRecipient := common.BytesToAddress(params.Message.ProposerFeeRecipient[:])
	expectedProfit := params.Message.Value.BigInt()

	var vmconfig vm.Config

	err = api.eth.BlockChain().ValidatePayload(block, feeRecipient, expectedProfit, params.RegisteredGasLimit, vmconfig)
	if err != nil {
		log.Error("invalid payload", "hash", payload.BlockHash.String(), "number", payload.BlockNumber, "parentHash", payload.ParentHash.String(), "err", err)
		return err
	}

	log.Info("validated block", "hash", block.Hash(), "number", block.NumberU64(), "parentHash", block.ParentHash())
	return nil
}

type BuilderBlockValidationRequestV2 struct {
	capellaapi.SubmitBlockRequest
	RegisteredGasLimit uint64 `json:"registered_gas_limit,string"`
}

type registeredGasLimit struct {
	RegisteredGasLimit uint64 `json:"registered_gas_limit,string"`
}

func (bbvr *BuilderBlockValidationRequestV2) UnmarshalJSON(data []byte) error {
	req := &capellaapi.SubmitBlockRequest{}
	err := json.Unmarshal(data, req)
	if err != nil {
		return fmt.Errorf("could not unmarshal SubmitBlockRequest: %w", err)
	}
	gl := &registeredGasLimit{}
	err = json.Unmarshal(data, gl)
	if err != nil {
		return fmt.Errorf("could not unmarshal RegisteredGasLimit: %w", err)
	}

	bbvr.SubmitBlockRequest = *req
	bbvr.RegisteredGasLimit = gl.RegisteredGasLimit
	return nil
}

func (api *BlockValidationAPI) ValidateBuilderSubmissionV2(params *BuilderBlockValidationRequestV2) error {
	// TODO: fuzztest, make sure the validation is sound
	// TODO: handle context!
	if params.ExecutionPayload == nil {
		return errors.New("nil execution payload")
	}
	payload := params.ExecutionPayload
	block, err := engine.ExecutionPayloadV2ToBlock(payload)
	if err != nil {
		return err
	}

	// validated at the relay
	// isShanghai := api.eth.BlockChain().Config().IsShanghai(params.ExecutionPayload.Timestamp)
	// if err := verifyWithdrawals(block.Withdrawals(), params.WithdrawalsRoot, isShanghai); err != nil {
	// 	return err
	// }

	if params.Message.ParentHash != phase0.Hash32(block.ParentHash()) {
		return fmt.Errorf("incorrect ParentHash %s, expected %s", params.Message.ParentHash.String(), block.ParentHash().String())
	}

	if params.Message.BlockHash != phase0.Hash32(block.Hash()) {
		return fmt.Errorf("incorrect BlockHash %s, expected %s", params.Message.BlockHash.String(), block.Hash().String())
	}

	if params.Message.GasLimit != block.GasLimit() {
		return fmt.Errorf("incorrect GasLimit %d, expected %d", params.Message.GasLimit, block.GasLimit())
	}

	if params.Message.GasUsed != block.GasUsed() {
		return fmt.Errorf("incorrect GasUsed %d, expected %d", params.Message.GasUsed, block.GasUsed())
	}

	feeRecipient := common.BytesToAddress(params.Message.ProposerFeeRecipient[:])
	expectedProfit := params.Message.Value.ToBig()

	var vmconfig vm.Config

	err = api.eth.BlockChain().ValidatePayload(block, feeRecipient, expectedProfit, params.RegisteredGasLimit, vmconfig)
	if err != nil {
		log.Error("invalid payload", "hash", payload.BlockHash.String(), "number", payload.BlockNumber, "parentHash", payload.ParentHash.String(), "err", err)
		return err
	}

	log.Info("validated block", "hash", block.Hash(), "number", block.NumberU64(), "parentHash", block.ParentHash())
	return nil
}

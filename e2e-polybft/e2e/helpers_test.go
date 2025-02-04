package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"testing"

	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi/artifact"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
	ethgow "github.com/umbracle/ethgo/wallet"
)

type e2eStateProvider struct {
	txRelayer txrelayer.TxRelayer
}

func (s *e2eStateProvider) Call(contractAddr ethgo.Address, input []byte, opts *contract.CallOpts) ([]byte, error) {
	response, err := s.txRelayer.Call(ethgo.Address(types.ZeroAddress), contractAddr, input)
	if err != nil {
		return nil, err
	}

	return hex.DecodeHex(response)
}

func (s *e2eStateProvider) Txn(ethgo.Address, ethgo.Key, []byte) (contract.Txn, error) {
	return nil, errors.New("send txn is not supported")
}

// isExitEventProcessed queries ExitHelper and as a result returns indication whether given exit event id is processed
func isExitEventProcessed(exitEventID uint64, exitHelper ethgo.Address, rootTxRelayer txrelayer.TxRelayer) (bool, error) {
	result, err := ABICall(
		rootTxRelayer,
		contractsapi.ExitHelper,
		exitHelper,
		ethgo.ZeroAddress,
		"processedExits",
		new(big.Int).SetUint64(exitEventID))
	if err != nil {
		return false, err
	}

	isProcessed, err := types.ParseUint64orHex(&result)
	if err != nil {
		return false, err
	}

	return isProcessed == uint64(1), nil
}

// getRootchainValidators queries rootchain validator set
func getRootchainValidators(relayer txrelayer.TxRelayer, checkpointManagerAddr ethgo.Address) ([]*polybft.ValidatorInfo, error) {
	validatorsCountRaw, err := ABICall(relayer, contractsapi.CheckpointManager,
		checkpointManagerAddr, ethgo.ZeroAddress, "currentValidatorSetLength")
	if err != nil {
		return nil, err
	}

	validatorsCount, err := types.ParseUint64orHex(&validatorsCountRaw)
	if err != nil {
		return nil, err
	}

	currentValidatorSetMethod := contractsapi.CheckpointManager.Abi.GetMethod("currentValidatorSet")
	validators := make([]*polybft.ValidatorInfo, validatorsCount)

	for i := 0; i < int(validatorsCount); i++ {
		validatorRaw, err := ABICall(relayer, contractsapi.CheckpointManager,
			checkpointManagerAddr, ethgo.ZeroAddress, "currentValidatorSet", i)
		if err != nil {
			return nil, err
		}

		validatorSetRaw, err := hex.DecodeString(validatorRaw[2:])
		if err != nil {
			return nil, err
		}

		decodedResults, err := currentValidatorSetMethod.Outputs.Decode(validatorSetRaw)
		if err != nil {
			return nil, err
		}

		results, ok := decodedResults.(map[string]interface{})
		if !ok {
			return nil, errors.New("failed to decode validator")
		}

		//nolint:forcetypeassert
		validators[i] = &polybft.ValidatorInfo{
			Address:    results["_address"].(ethgo.Address),
			TotalStake: results["votingPower"].(*big.Int),
		}
	}

	return validators, nil
}

func ABICall(relayer txrelayer.TxRelayer, artifact *artifact.Artifact, contractAddress ethgo.Address, senderAddr ethgo.Address, method string, params ...interface{}) (string, error) {
	input, err := artifact.Abi.GetMethod(method).Encode(params)
	if err != nil {
		return "", err
	}

	return relayer.Call(senderAddr, contractAddress, input)
}

func ABITransaction(relayer txrelayer.TxRelayer, key ethgo.Key, artifact *artifact.Artifact, contractAddress ethgo.Address, method string, params ...interface{}) (*ethgo.Receipt, error) {
	input, err := artifact.Abi.GetMethod(method).Encode(params)
	if err != nil {
		return nil, err
	}

	return relayer.SendTransaction(&ethgo.Transaction{
		To:    &contractAddress,
		Input: input,
	}, key)
}

func sendExitTransaction(
	sidechainKey *ethgow.Key,
	rootchainKey ethgo.Key,
	proof types.Proof,
	checkpointBlock uint64,
	stateSenderData []byte,
	l1ExitTestAddr,
	exitHelperAddr ethgo.Address,
	l1TxRelayer txrelayer.TxRelayer,
	exitEventID uint64) (bool, error) {
	var exitEventAPI contractsapi.L2StateSyncedEvent

	proofExitEventEncoded, err := exitEventAPI.Encode(&polybft.ExitEvent{
		ID:       exitEventID,
		Sender:   sidechainKey.Address(),
		Receiver: l1ExitTestAddr,
		Data:     stateSenderData,
	})
	if err != nil {
		return false, err
	}

	leafIndex, ok := proof.Metadata["LeafIndex"].(float64)
	if !ok {
		return false, fmt.Errorf("could not get leaf index from exit event proof. Leaf from proof: %v", proof.Metadata["LeafIndex"])
	}

	receipt, err := ABITransaction(l1TxRelayer, rootchainKey, contractsapi.ExitHelper, exitHelperAddr,
		"exit",
		big.NewInt(int64(checkpointBlock)),
		uint64(leafIndex),
		proofExitEventEncoded,
		proof.Data,
	)

	if err != nil {
		return false, err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return false, errors.New("transaction execution failed")
	}

	return isExitEventProcessed(exitEventID, exitHelperAddr, l1TxRelayer)
}

func getExitProof(rpcAddress string, exitID uint64) (types.Proof, error) {
	query := struct {
		Jsonrpc string   `json:"jsonrpc"`
		Method  string   `json:"method"`
		Params  []string `json:"params"`
		ID      int      `json:"id"`
	}{
		"2.0",
		"bridge_generateExitProof",
		[]string{fmt.Sprintf("0x%x", exitID)},
		1,
	}

	d, err := json.Marshal(query)
	if err != nil {
		return types.Proof{}, err
	}

	resp, err := http.Post(rpcAddress, "application/json", bytes.NewReader(d))
	if err != nil {
		return types.Proof{}, err
	}

	s, err := io.ReadAll(resp.Body)
	if err != nil {
		return types.Proof{}, err
	}

	rspProof := struct {
		Result types.Proof `json:"result"`
	}{}

	err = json.Unmarshal(s, &rspProof)
	if err != nil {
		return types.Proof{}, err
	}

	return rspProof.Result, nil
}

// checkStateSyncResultLogs is helper function which parses given StateSyncResultEvent event's logs,
// extracts status topic value and makes assertions against it.
func checkStateSyncResultLogs(
	t *testing.T,
	logs []*ethgo.Log,
	expectedCount int,
) {
	t.Helper()
	require.Len(t, logs, expectedCount)

	var stateSyncResultEvent contractsapi.StateSyncResultEvent
	for _, log := range logs {
		doesMatch, err := stateSyncResultEvent.ParseLog(log)
		require.True(t, doesMatch)
		require.NoError(t, err)

		t.Logf("Block Number=%d, Decoded Log=%+v\n", log.BlockNumber, stateSyncResultEvent)

		require.True(t, stateSyncResultEvent.Status)
	}
}

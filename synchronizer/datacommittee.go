package synchronizer

import (
	"fmt"
	"math/big"
	"math/rand"

	"github.com/0xPolygon/cdk-validium-node/log"
	"github.com/0xPolygon/cdk-validium-node/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const unexpectedHashTemplate = "missmatch on transaction data for batch num %d. Expected hash %s, actual hash: %s"

func (s *ClientSynchronizer) loadCommittee() error {
	committee, err := s.etherMan.GetCurrentDataCommittee()
	if err != nil {
		return err
	}
	selectedCommitteeMember := -1
	if committee != nil {
		s.committeeMembers = committee.Members
		if len(committee.Members) > 0 {
			selectedCommitteeMember = rand.Intn(len(committee.Members)) //nolint:gosec
		}
	}
	s.selectedCommitteeMember = selectedCommitteeMember
	return nil
}

// Here we cover expectedTransactionsHash being either:
// 1. the composite hash for NEAR da [tx_id ++ commitment] (64bytes)
// 2. the transaction hash for ethereum
func (s *ClientSynchronizer) getBatchL2Data(batchNum uint64, expectedTransactionsHash common.Hash, expectedDaCommitment []byte) ([]byte, error) {
	log.Warnf("trying to get data from local DB for batch num %d and expected hash %s, NEAR data commitment %s", batchNum, expectedTransactionsHash, common.Bytes2Hex(expectedDaCommitment))
	found := true
	transactionsData, err := s.state.GetBatchL2DataByNumber(s.ctx, batchNum, nil)
	if err != nil {
		if err == state.ErrNotFound {
			found = false
		} else {
			return nil, fmt.Errorf("failed to get batch data from state for batch num %d: %w", batchNum, err)
		}
	}
	actualTransactionsHash := crypto.Keccak256Hash(transactionsData)
	if !found || expectedTransactionsHash != actualTransactionsHash {
		if found {
			log.Warnf(unexpectedHashTemplate, batchNum, expectedTransactionsHash, actualTransactionsHash)
		}

		// TODO: THIS IS A HACK TO ALWAYS GET FROM NEAR - DONT USE IN PROD
		testAlwaysGetFromNear := true

		if !s.isTrustedSequencer && !testAlwaysGetFromNear {
			log.Info("trying to get data from trusted sequencer")
			// TODO[optimisation]: shortcut read from NEAR first
			data, err := s.getDataFromTrustedSequencer(batchNum, expectedTransactionsHash)
			if err != nil {
				log.Error(err)
			} else {
				return data, nil
			}
		}

		log.Info("trying to get data from NEAR data availability")
		data, err := s.getDataFromCommittee(batchNum, expectedDaCommitment)
		if err != nil {
			log.Error(err)
			if s.isTrustedSequencer {
				return nil, fmt.Errorf("data not found on the local DB nor on NEAR data availability")
			} else {
				return nil, fmt.Errorf("data not found on the local DB, nor from the trusted sequencer nor on NEAR data availability")
			}
		}
		return data, nil
	}
	return transactionsData, nil
}

func (s *ClientSynchronizer) getDataFromCommittee(batchNum uint64, daCommitment []byte) ([]byte, error) {
	data, err := s.daClient.Get(daCommitment, uint32(batchNum))
	if err != nil {
		log.Warnf("error getting data from NEAR data availability: %s", err)
		return nil, err
	} else {
		log.Infof("Got data from NEAR")
		return data, nil
	}
}

func (s *ClientSynchronizer) getDataFromTrustedSequencer(batchNum uint64, expectedTransactionsHash common.Hash) ([]byte, error) {
	b, err := s.zkEVMClient.BatchByNumber(s.ctx, big.NewInt(int64(batchNum)))
	if err != nil {
		return nil, fmt.Errorf("failed to get batch num %d from trusted sequencer: %w", batchNum, err)
	}
	actualTransactionsHash := crypto.Keccak256Hash(b.BatchL2Data)
	if expectedTransactionsHash != actualTransactionsHash {
		return nil, fmt.Errorf(
			unexpectedHashTemplate, batchNum, expectedTransactionsHash, actualTransactionsHash,
		)
	}
	return b.BatchL2Data, nil
}

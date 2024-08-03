package main

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/stretchr/testify/require"
)

func TestDecodeExecutingMessage(t *testing.T) {

	t.Run("Test successful decoding", func(t *testing.T) {
		number := Identifier{Origin: common.Address{}, BlockNumber: 1, LogIndex: 1, Timestamp: uint64(time.Now().Unix()), ChainId: 1}
		// Encode number to bytes using RLP encoding
		data, err := rlp.EncodeToBytes(number)
		require.NoError(t, err)

		exampleLog := &types.Log{
			Data: data,
		}

		_, err = decodeExecutingMessage(exampleLog)
		require.NoError(t, err)
	})
}

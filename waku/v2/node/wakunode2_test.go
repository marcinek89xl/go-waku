package node

import (
	"bytes"
	"context"
	"math/big"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/status-im/go-waku/tests"
	"github.com/status-im/go-waku/waku/persistence"
	"github.com/status-im/go-waku/waku/persistence/sqlite"
	"github.com/status-im/go-waku/waku/v2/protocol/filter"
	"github.com/status-im/go-waku/waku/v2/protocol/relay"
	"github.com/status-im/go-waku/waku/v2/protocol/store"
	"github.com/status-im/go-waku/waku/v2/utils"
	"github.com/stretchr/testify/require"
)

func TestWakuNode2(t *testing.T) {
	hostAddr, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")

	key, err := tests.RandomHex(32)
	require.NoError(t, err)
	prvKey, err := crypto.HexToECDSA(key)
	require.NoError(t, err)

	ctx := context.Background()

	wakuNode, err := New(ctx,
		WithPrivateKey(prvKey),
		WithHostAddress(hostAddr),
		WithWakuRelay(),
	)
	require.NoError(t, err)

	err = wakuNode.Start()
	defer wakuNode.Stop()

	require.NoError(t, err)
}

func int2Bytes(i int) []byte {
	if i > 0 {
		return append(big.NewInt(int64(i)).Bytes(), byte(1))
	}
	return append(big.NewInt(int64(i)).Bytes(), byte(0))
}

func Test5000(t *testing.T) {
	maxMsgs := 5000
	maxMsgBytes := int2Bytes(maxMsgs)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	hostAddr1, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	key1, _ := tests.RandomHex(32)
	prvKey1, _ := crypto.HexToECDSA(key1)

	hostAddr2, _ := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	key2, _ := tests.RandomHex(32)
	prvKey2, _ := crypto.HexToECDSA(key2)

	wakuNode1, err := New(ctx,
		WithPrivateKey(prvKey1),
		WithHostAddress(hostAddr1),
		WithWakuRelay(),
	)
	require.NoError(t, err)
	err = wakuNode1.Start()
	require.NoError(t, err)
	defer wakuNode1.Stop()

	wakuNode2, err := New(ctx,
		WithPrivateKey(prvKey2),
		WithHostAddress(hostAddr2),
		WithWakuRelay(),
	)
	require.NoError(t, err)
	err = wakuNode2.Start()
	require.NoError(t, err)
	defer wakuNode2.Stop()

	err = wakuNode2.DialPeer(ctx, wakuNode1.ListenAddresses()[0].String())
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	sub1, err := wakuNode1.Relay().Subscribe(ctx)
	require.NoError(t, err)
	sub2, err := wakuNode1.Relay().Subscribe(ctx)
	require.NoError(t, err)

	wg := sync.WaitGroup{}
	wg.Add(3)
	go func() {
		defer wg.Done()

		ticker := time.NewTimer(60 * time.Second)
		defer ticker.Stop()
		var count int
		for {
			select {
			case <-ticker.C:
				require.Fail(t, "Timeout Sub1", count)
			case msg := <-sub1.C:
				if bytes.Equal(msg.Message().Payload, maxMsgBytes) {
					return
				}
				count++
			}
		}
	}()

	go func() {
		defer wg.Done()

		ticker := time.NewTimer(60 * time.Second)
		defer ticker.Stop()
		var count int
		for {
			select {
			case <-ticker.C:
				require.Fail(t, "Timeout Sub2", count)
			case msg := <-sub2.C:
				if bytes.Equal(msg.Message().Payload, maxMsgBytes) {
					return
				}
				count++
			}
		}
	}()

	go func() {
		defer wg.Done()
		ticker := time.NewTimer(60 * time.Second)
		defer ticker.Stop()
		for i := 1; i <= maxMsgs; i++ {
			select {
			case <-ticker.C:
				require.Fail(t, "Timeout Publish", i)
			default:
				msg := createTestMsg(0)
				msg.Payload = int2Bytes(i)
				msg.Timestamp = int64(i)
				if err := wakuNode2.Publish(ctx, msg); err != nil {
					require.Fail(t, "Could not publish all messages", i)
				}
			}
		}
	}()

	wg.Wait()

}

func TestDecoupledStoreFromRelay(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// NODE1: Relay Node + Filter Server
	hostAddr1, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	wakuNode1, err := New(ctx,
		WithHostAddress(hostAddr1),
		WithWakuRelay(),
		WithWakuFilter(true),
	)
	require.NoError(t, err)
	err = wakuNode1.Start()
	require.NoError(t, err)
	defer wakuNode1.Stop()

	// NODE2: Filter Client/Store
	db, err := sqlite.NewDB(":memory:")
	require.NoError(t, err)
	dbStore, err := persistence.NewDBStore(utils.Logger(), persistence.WithDB(db))
	require.NoError(t, err)

	hostAddr2, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	wakuNode2, err := New(ctx,
		WithHostAddress(hostAddr2),
		WithWakuFilter(false),
		WithWakuStore(true, false),
		WithMessageProvider(dbStore),
	)
	require.NoError(t, err)
	err = wakuNode2.Start()
	require.NoError(t, err)
	defer wakuNode2.Stop()

	err = wakuNode2.DialPeerWithMultiAddress(ctx, wakuNode1.ListenAddresses()[0])
	require.NoError(t, err)

	time.Sleep(2 * time.Second)

	_, filter, err := wakuNode2.Filter().Subscribe(ctx, filter.ContentFilter{
		Topic: string(relay.DefaultWakuTopic),
	})
	require.NoError(t, err)

	// Sleep to make sure the filter is subscribed
	time.Sleep(1 * time.Second)

	// Send MSG1 on NODE1
	msg := createTestMsg(0)
	msg.Payload = []byte{1, 2, 3, 4, 5}
	msg.Timestamp = utils.GetUnixEpoch()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		// MSG1 should be pushed in NODE2 via filter
		defer wg.Done()
		env := <-filter.Chan
		require.Equal(t, msg.Timestamp, env.Message().Timestamp)
	}()

	time.Sleep(500 * time.Millisecond)

	if err := wakuNode1.Publish(ctx, msg); err != nil {
		require.Fail(t, "Could not publish all messages")
	}

	wg.Wait()

	// NODE3: Query from NODE2
	hostAddr3, err := net.ResolveTCPAddr("tcp", "0.0.0.0:0")
	require.NoError(t, err)
	wakuNode3, err := New(ctx,
		WithHostAddress(hostAddr3),
		WithWakuFilter(false),
	)
	require.NoError(t, err)
	err = wakuNode3.Start()
	require.NoError(t, err)
	defer wakuNode3.Stop()

	err = wakuNode3.DialPeerWithMultiAddress(ctx, wakuNode2.ListenAddresses()[0])
	require.NoError(t, err)

	// NODE2 should have returned the message received via filter
	result, err := wakuNode3.Store().Query(ctx, store.Query{})
	require.NoError(t, err)
	require.Len(t, result.Messages, 1)
	require.Equal(t, msg.Timestamp, result.Messages[0].Timestamp)
}

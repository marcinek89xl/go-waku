package relay

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/status-im/go-waku/tests"
	"github.com/status-im/go-waku/waku/v2/protocol/pb"
	"github.com/stretchr/testify/require"
)

func TestWakuRelay(t *testing.T) {
	var testTopic Topic = "/waku/2/go/relay/test"

	port, err := tests.FindFreePort(t, "", 5)
	require.NoError(t, err)

	host, err := tests.MakeHost(context.Background(), port, rand.Reader)
	require.NoError(t, err)

	relay, err := NewWakuRelay(context.Background(), host)
	defer relay.Stop()
	require.NoError(t, err)

	sub, isNew, err := relay.Subscribe(testTopic)
	defer sub.Cancel()
	require.NoError(t, err)
	require.True(t, isNew)

	_, isNew, err = relay.Subscribe(testTopic)
	require.NoError(t, err)
	require.False(t, isNew)

	topics := relay.Topics()
	require.Equal(t, 1, len(topics))
	require.Equal(t, testTopic, topics[0])

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()

		_, err := sub.Next(ctx)
		require.NoError(t, err)
	}()

	msg := &pb.WakuMessage{
		Payload:      []byte{1},
		Version:      0,
		ContentTopic: "test",
		Timestamp:    0,
	}
	_, err = relay.Publish(context.Background(), msg, &testTopic)
	require.NoError(t, err)

	<-ctx.Done()
}
package filter

import (
	"testing"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/test"
	"github.com/status-im/go-waku/waku/v2/protocol/pb"
	"github.com/stretchr/testify/assert"
)

const TOPIC = "/test/topic"

func createPeerId(t *testing.T) peer.ID {
	peerId, err := test.RandPeerID()
	assert.NoError(t, err)
	return peerId
}

func firstSubscriber(subs *Subscribers, contentTopic string) *Subscriber {
	for sub := range subs.Items(&contentTopic) {
		return &sub
	}
	return nil
}

func TestAppend(t *testing.T) {
	subs := NewSubscribers(10 * time.Second)
	peerId := createPeerId(t)
	contentTopic := "topic1"
	request := pb.FilterRequest{
		Subscribe:      true,
		Topic:          TOPIC,
		ContentFilters: []*pb.FilterRequest_ContentFilter{{ContentTopic: contentTopic}},
	}
	subs.Append(Subscriber{peerId, "request_1", request})

	sub := firstSubscriber(subs, contentTopic)
	assert.NotNil(t, sub)
}

func TestRemove(t *testing.T) {
	subs := NewSubscribers(10 * time.Second)
	peerId := createPeerId(t)
	contentTopic := "topic1"
	request := pb.FilterRequest{
		Subscribe:      true,
		Topic:          TOPIC,
		ContentFilters: []*pb.FilterRequest_ContentFilter{{ContentTopic: contentTopic}},
	}
	subs.Append(Subscriber{peerId, "request_1", request})
	subs.RemoveContentFilters(peerId, request.ContentFilters)

	sub := firstSubscriber(subs, contentTopic)
	assert.Nil(t, sub)
}

func TestRemovePartial(t *testing.T) {
	subs := NewSubscribers(10 * time.Second)
	peerId := createPeerId(t)
	topic1 := "topic1"
	topic2 := "topic2"
	request := pb.FilterRequest{
		Subscribe:      true,
		Topic:          TOPIC,
		ContentFilters: []*pb.FilterRequest_ContentFilter{{ContentTopic: topic1}, {ContentTopic: topic2}},
	}
	subs.Append(Subscriber{peerId, "request_1", request})
	subs.RemoveContentFilters(peerId, []*pb.FilterRequest_ContentFilter{{ContentTopic: topic1}})

	sub := firstSubscriber(subs, topic2)
	assert.NotNil(t, sub)
	assert.Len(t, sub.filter.ContentFilters, 1)
}

func TestRemoveBogus(t *testing.T) {
	subs := NewSubscribers(10 * time.Second)
	peerId := createPeerId(t)
	contentTopic := "topic1"
	request := pb.FilterRequest{
		Subscribe:      true,
		Topic:          TOPIC,
		ContentFilters: []*pb.FilterRequest_ContentFilter{{ContentTopic: contentTopic}},
	}
	subs.Append(Subscriber{peerId, "request_1", request})
	subs.RemoveContentFilters(peerId, []*pb.FilterRequest_ContentFilter{{ContentTopic: "does not exist"}, {ContentTopic: contentTopic}})

	sub := firstSubscriber(subs, contentTopic)
	assert.Nil(t, sub)
}

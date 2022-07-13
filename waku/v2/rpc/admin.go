package rpc

import (
	"net/http"

	ma "github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"

	"github.com/status-im/go-waku/waku/v2/node"
	"github.com/status-im/go-waku/waku/v2/protocol/filter"
	"github.com/status-im/go-waku/waku/v2/protocol/lightpush"
	"github.com/status-im/go-waku/waku/v2/protocol/relay"
	"github.com/status-im/go-waku/waku/v2/protocol/store"
)

type AdminService struct {
	node *node.WakuNode
	log  *zap.Logger
}

type GetPeersArgs struct {
}

type PeersArgs struct {
	Peers []string `json:"peers,omitempty"`
}

type PeerReply struct {
	Multiaddr string `json:"mutliaddr,omitempty"`
	Protocol  string `json:"protocol,omitempty"`
	Connected bool   `json:"connected,omitempty"`
}

type PeersReply struct {
	Peers []PeerReply `json:"peers,omitempty"`
}

func (a *AdminService) PostV1Peers(req *http.Request, args *PeersArgs, reply *SuccessReply) error {
	for _, peer := range args.Peers {
		addr, err := ma.NewMultiaddr(peer)
		if err != nil {
			a.log.Error("building multiaddr", zap.Error(err))
			return err
		}

		err = a.node.DialPeerWithMultiAddress(req.Context(), addr)
		if err != nil {
			a.log.Error("dialing peers", zap.Error(err))
			return err
		}
	}

	*reply = true
	return nil
}

func isWakuProtocol(protocol string) bool {
	return protocol == string(filter.FilterID_v20beta1) || protocol == string(relay.WakuRelayID_v200) || protocol == string(lightpush.LightPushID_v20beta1) || protocol == string(store.StoreID_v20beta4)
}

func (a *AdminService) GetV1Peers(req *http.Request, args *GetPeersArgs, reply *PeersReply) error {
	peers, err := a.node.Peers()
	if err != nil {
		a.log.Error("getting peers", zap.Error(err))
		return nil
	}
	for _, peer := range peers {
		for _, addr := range peer.Addrs {
			for _, proto := range peer.Protocols {
				if !isWakuProtocol(proto) {
					continue
				}
				reply.Peers = append(reply.Peers, PeerReply{
					Multiaddr: addr.String(),
					Protocol:  proto,
					Connected: peer.Connected,
				})
			}
		}
	}
	return nil
}

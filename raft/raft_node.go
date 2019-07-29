//Created by zhbinary on 2019-07-22.
//Email: zhbinary@gmail.com
package raft

import (
	"quick-raft/pb"
	"time"
)

const (
	TickTimeInterval = 200 * time.Millisecond
)

type RaftNode struct {
	id        uint64
	elector   *Elector
	InboundC  chan *pb.Message
	OutboundC chan *pb.Message
	done      chan struct{}
}

func NewRaftNode(id uint64, peers [] uint64, canLeader bool) (node *RaftNode) {
	node = &RaftNode{id: id, InboundC: make(chan *pb.Message, 128), OutboundC: make(chan *pb.Message, 128), elector: NewElector(id, peers, canLeader), done: make(chan struct{})}
	return
}

func (this *RaftNode) Start() {
	for {
		select {
		case msg := <-this.InboundC:
			this.elector.Handle(msg)
		case <-time.After(TickTimeInterval):
			this.elector.Tick()
		case out := <-this.elector.OutboundC:
			this.OutboundC <- out
		case <-this.done:
			return
		}
	}
}

func (this *RaftNode) Close() {
	close(this.done)
}

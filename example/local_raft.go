//Created by zhbinary on 2019-07-22.
//Email: zhbinary@gmail.com
package main

import (
	"os"
	"os/signal"
	"quick-raft/pb"
	"quick-raft/raft"
	"syscall"
)

func main() {
	var nodes [] *raft.RaftNode

	// Init raft nodes
	ids := []uint64{1, 2, 3}
	for n, id := range ids {
		if n == 0 {
			nodes = append(nodes, raft.NewRaftNode(id, ids, true))
		} else {
			nodes = append(nodes, raft.NewRaftNode(id, ids, true))
		}
	}

	outC := make(chan *pb.Message, 128)
	for _, node := range nodes {
		go func(n *raft.RaftNode) {
			for {
				msg := <-n.OutboundC
				outC <- msg
			}
		}(node)
	}

	go func() {
		for {
			select {
			case msg := <-outC:
				nodes[msg.To-1].InboundC <- msg
			}
		}
	}()

	for _, node := range nodes {
		go node.Start()
	}

	done := make(chan os.Signal)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done
}

//Created by zhbinary on 2019-07-18.
//Email: zhbinary@gmail.com
package raft

import (
	"go.uber.org/zap"
	"math/rand"
	"quick-raft/pb"
	"quick-raft/utils"
	"time"
)

const (
	HeartBeatIntervalMs = 1000 * time.Millisecond
	SelectTimeout       = 10 * HeartBeatIntervalMs

	Follower = iota
	Candidate
	Leader

	Zero    = 0
	Nil     = -1
	TagRaft = "TagElector"
)

type Elector struct {
	id                        uint64
	leaderId                  uint64
	state                     int
	term                      uint64
	peers                     []uint64
	lastLogIndex              uint64
	lastLogTerm               uint64
	electionElapsed           int
	heartbeatElapsed          int
	electionTimeout           int
	heartbeatTimeout          int
	randomizedElectionTimeout int
	rand                      *rand.Rand
	OutboundC                 chan *pb.Message
	Tick                      func()
	Handle                    func(msg *pb.Message)
	voteFor                   uint64
	heartbeatRsps             map[uint64][]*pb.Message
	voteRsps                  map[uint64][]*pb.Message
	canLeader                 bool
	isLeader                  bool
}

func NewElector(id uint64, peers [] uint64, canLeader bool) (raft *Elector) {
	raft = &Elector{id: id, peers: peers, rand: rand.New(rand.NewSource(time.Now().UnixNano())), state: Follower, heartbeatTimeout: 1, electionTimeout: 10, OutboundC: make(chan *pb.Message, 128),
		heartbeatRsps: make(map[uint64][]*pb.Message), voteRsps: make(map[uint64][]*pb.Message), canLeader: canLeader}
	raft.becomeFollower(0, Zero)
	return
}

func (this *Elector) tickElection() bool {
	this.electionElapsed++
	if this.isElectionTimeout() {
		this.electionElapsed = 0
		return true
	}
	return false
}

func (this *Elector) tickAsFollow() {
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "tickAsFollow"))
	if this.tickElection() && this.canLeader {
		this.becomeCandidate()
	}
}

func (this *Elector) tickAsCandidate() {
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "tickAsCandidate"))
	if this.tickElection() {
		this.becomeCandidate()
	}
	this.checkVotes()
}

func (this *Elector) tickAsLeader() {
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "tickAsLeader"))
	this.heartbeatElapsed++
	if this.heartbeatElapsed >= this.heartbeatTimeout {
		this.heartbeatElapsed = 0
		this.sendHeartBeat()
	}
	this.checkHeartbeats()
}

func (this *Elector) isElectionTimeout() bool {
	return this.electionElapsed >= this.randomizedElectionTimeout
}

func (this *Elector) resetRandomizedElectionTimeout() {
	this.randomizedElectionTimeout = this.electionTimeout + this.rand.Intn(this.electionTimeout)
}

func (this *Elector) becomeFollower(term uint64, leaderId uint64) {
	this.state = Follower
	this.Tick = this.tickAsFollow
	this.Handle = this.handleAsFollower
	this.reset(term)
	this.leaderId = leaderId
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "becomeFollower"), zap.Uint64("term", this.term), zap.Uint64("leaderId", this.leaderId))
}

func (this *Elector) becomeCandidate() {
	this.state = Candidate
	this.Tick = this.tickAsCandidate
	this.Handle = this.handleAsCandidate
	this.reset(this.term + 1)
	this.voteFor = this.id
	this.leaderId = this.id
	this.sendVote()
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "becomeCandidate"), zap.Uint64("term", this.term), zap.Uint64("leaderId", this.leaderId))
}

func (this *Elector) becomeLeader() {
	this.isLeader = true
	this.state = Leader
	this.Tick = this.tickAsLeader
	this.Handle = this.handleAsLeader
	this.reset(this.term)
	this.voteFor = this.id
	this.leaderId = this.id
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "becomeLeader"), zap.Uint64("term", this.term), zap.Uint64("leaderId", this.leaderId))
	this.sendHeartBeat()
}

func (this *Elector) handleAsFollower(msg *pb.Message) {
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "handleAsFollower"), zap.Reflect("req", msg))
	switch msg.Type {
	case pb.MessageType_MsgHeartbeatReq:
		this.resetTimeElapsed()
		rsp := &pb.Message{Type: pb.MessageType_MsgHeartbeatRsp, Term: this.term, LeaderId: this.leaderId, From: this.id, To: msg.From}
		if this.term > msg.Term {
			// Reject
			rsp.Success = false
		} else if this.term == msg.Term {
			if msg.LeaderId == this.leaderId {
				rsp.Success = true
				rsp.Term = msg.Term
			} else {
				// Reject
				rsp.Success = false
			}
		} else {
			this.term = msg.Term
			rsp.Term = msg.Term
			this.leaderId = msg.LeaderId
			rsp.Success = true
		}
		this.OutboundC <- rsp
	case pb.MessageType_MsgVoteReq:
		this.resetTimeElapsed()
		rsp := &pb.Message{Type: pb.MessageType_MsgVoteRsp, Term: this.term, LeaderId: this.leaderId, From: this.id, To: msg.From}
		if this.term <= msg.Term {
			if this.voteFor == Zero {
				this.term = msg.Term
				rsp.Term = msg.Term
				rsp.LeaderId = msg.LeaderId
				rsp.Success = true
				this.voteFor = msg.LeaderId
			}
		} else {
			rsp.Success = false
		}
		this.OutboundC <- rsp
	default:
		this.handle(msg)
	}
}

func (this *Elector) handleAsCandidate(msg *pb.Message) {
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "handleAsCandidate"), zap.Reflect("req", msg))
	switch msg.Type {
	case pb.MessageType_MsgHeartbeatReq:
		rsp := &pb.Message{Type: pb.MessageType_MsgHeartbeatRsp, Term: this.term, LeaderId: this.leaderId, From: this.id, To: msg.From}
		if this.term <= msg.Term {
			this.becomeFollower(msg.Term, msg.LeaderId)
			rsp.Success = true
		}
		this.OutboundC <- rsp
	case pb.MessageType_MsgVoteReq:
		rsp := &pb.Message{Type: pb.MessageType_MsgVoteRsp, Term: this.term, LeaderId: this.leaderId, From: this.id, To: msg.From}
		if this.term < msg.Term {
			rsp.Success = true
			this.becomeFollower(msg.Term, msg.LeaderId)
		}
		this.OutboundC <- rsp
	default:
		this.handle(msg)
	}
}

func (this *Elector) handleAsLeader(msg *pb.Message) {
	utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "handleAsLeader"), zap.Reflect("req", msg))
	switch msg.Type {
	case pb.MessageType_MsgHeartbeatReq:
		rsp := &pb.Message{Type: pb.MessageType_MsgHeartbeatRsp, Term: this.term, LeaderId: this.leaderId, From: this.id, To: msg.From}
		if this.term < msg.Term {
			rsp.Success = true
			this.becomeFollower(msg.Term, msg.LeaderId)
		}
		this.OutboundC <- rsp
	case pb.MessageType_MsgVoteReq:
		rsp := &pb.Message{Type: pb.MessageType_MsgVoteRsp, Term: this.term, LeaderId: this.leaderId, From: this.id, To: msg.From}
		if this.term < msg.Term {
			rsp.Success = true
			this.becomeFollower(msg.Term, msg.LeaderId)
		}
		this.OutboundC <- rsp
	default:
		this.handle(msg)
	}
}

func (this *Elector) handle(msg interface{}) {
	rsp, ok := msg.(*pb.Message)
	if ok {
		switch rsp.Type {
		case pb.MessageType_MsgHeartbeatRsp:
			rsps := this.heartbeatRsps[rsp.Term]
			this.heartbeatRsps[this.term] = rsps
		case pb.MessageType_MsgVoteRsp:
			rsps := this.voteRsps[rsp.Term]
			rsps = append(rsps, rsp)
			this.voteRsps[this.term] = rsps
		default:
		}
	}
}

func (this *Elector) reset(term uint64) {
	this.isLeader = false
	this.term = term
	this.leaderId = Zero
	this.voteFor = Zero
	this.electionElapsed = Zero
	this.heartbeatElapsed = Zero
	this.lastLogTerm = Zero
	this.lastLogIndex = Zero
	this.resetRandomizedElectionTimeout()
}

func (this *Elector) resetTimeElapsed() {
	this.electionElapsed = Zero
	this.heartbeatElapsed = Zero
}

func (this *Elector) checkHeartbeats() {
	utils.Logger.Debug(TagRaft, zap.String("msg", "checkHeartbeats"))
	rsps := this.heartbeatRsps[this.term]
	if rsps != nil {
		for _, rsp := range rsps {
			if rsp.Success {
			} else {
				if this.term < rsp.Term {
					this.becomeFollower(rsp.Term, rsp.LeaderId)
				} else if this.term == rsp.Term {
					if this.leaderId != rsp.LeaderId {
						this.becomeFollower(this.term, Zero)
					}
				} else {
					return
				}
			}
		}
	}
}

func (this *Elector) checkVotes() {
	if this.isLeader {
		return
	}
	utils.Logger.Debug(TagRaft, zap.String("msg", "checkVotes"), zap.Reflect("voteRsps", this.voteRsps))
	rsps := this.voteRsps[this.term]
	if rsps != nil {
		var successCnt int
		for _, rsp := range rsps {
			if rsp.Success {
				successCnt++
			} else {
				if this.term < rsp.Term {
					this.becomeFollower(rsp.Term, rsp.LeaderId)
					return
				} else if this.term == rsp.Term {
					// Do nothing
				} else {
					// Do nothing
				}
			}
		}

		if this.isQuorum(successCnt) {
			this.becomeLeader()
		}
	}
}

func (this *Elector) sendHeartBeat() {
	for _, peer := range this.peers {
		if peer != this.id {
			utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "sendHeartBeat"), zap.Reflect("to", peer))
			req := &pb.Message{Type: pb.MessageType_MsgHeartbeatReq, Term: this.term, LeaderId: this.leaderId, From: this.id, To: peer}
			this.OutboundC <- req
		}
	}
}

func (this *Elector) sendVote() {
	for _, peer := range this.peers {
		if peer != this.id {
			utils.Logger.Debug(TagRaft, zap.Uint64("id", this.id), zap.String("msg", "sendVote"), zap.Reflect("to", peer))
			req := &pb.Message{Type: pb.MessageType_MsgVoteReq, Term: this.term, LeaderId: this.leaderId, From: this.id, To: peer}
			this.OutboundC <- req
		}
	}
}

//func (this *Elector) sendHeartBeat() {
// Invoke by leader
//var successCnt uint64
//done := make(chan struct{}, 1)
//for _, peer := range this.peers {
//go func() {
//	req := &proto.HeartbeatReq{Term: this.term, LeaderId: this.leaderId}
//	rsp := this.SendHeartbeatF(peer, req)
//	if rsp.Success {
//		atomic.AddUint64(&successCnt, 1)
//	}
//	if successCnt == uint64(len(this.peers)) {
//		done <- struct{}{}
//	}
//}()
//}

//select {
//case <-time.After(20 * time.Millisecond):
//	// Step down to follower if didn't take majority of response
//	if !this.isQuorum(int(successCnt)) {
//		this.becomeFollower()
//	}
//case <-done:
//}
//}

//func (this *Elector) sendVote() {
//	// Invoke by candidate
//	var successCnt uint64
//	done := make(chan struct{}, 1)
//	for _, peer := range this.peers {
//		go func() {
//			req := &proto.VoteReq{Term: this.term, LeaderId: this.leaderId}
//			rsp := this.SendVoteF(peer, req)
//			if rsp.Granted {
//				atomic.AddUint64(&successCnt, 1)
//			}
//			if successCnt == uint64(len(this.peers)) {
//				done <- struct{}{}
//			}
//		}()
//	}
//
//	select {
//	case <-time.After(20 * time.Millisecond):
//		// Step down to follower if didn't take majority of response
//		if !this.isQuorum(int(successCnt)) {
//			this.becomeFollower()
//		}
//	case <-done:
//	}
//}

func (this *Elector) isQuorum(cnt int) bool {
	return cnt >= len(this.peers)/2+1
}

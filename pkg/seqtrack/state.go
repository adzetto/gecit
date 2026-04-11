package seqtrack

import (
	"sync"
	"time"

	"github.com/boratanrikulu/gecit/pkg/capture"
)

type seqState struct {
	mu      sync.Mutex
	conns   map[uint16]capture.ConnectionEvent
	waiters map[uint16][]chan capture.ConnectionEvent
}

func newSeqState() *seqState {
	return &seqState{
		conns:   make(map[uint16]capture.ConnectionEvent),
		waiters: make(map[uint16][]chan capture.ConnectionEvent),
	}
}

func (s *seqState) store(evt capture.ConnectionEvent) {
	s.mu.Lock()
	waiters := s.waiters[evt.SrcPort]
	if len(waiters) > 0 {
		ch := waiters[0]
		if len(waiters) == 1 {
			delete(s.waiters, evt.SrcPort)
		} else {
			s.waiters[evt.SrcPort] = waiters[1:]
		}
		s.mu.Unlock()
		ch <- evt
		return
	}

	s.conns[evt.SrcPort] = evt
	s.mu.Unlock()
}

func (s *seqState) wait(localPort uint16, timeout time.Duration) *capture.ConnectionEvent {
	s.mu.Lock()
	if evt, ok := s.conns[localPort]; ok {
		delete(s.conns, localPort)
		s.mu.Unlock()
		return &evt
	}

	ch := make(chan capture.ConnectionEvent, 1)
	s.waiters[localPort] = append(s.waiters[localPort], ch)
	s.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case evt := <-ch:
		return &evt
	case <-timer.C:
		s.removeWaiter(localPort, ch)
		return nil
	}
}

func (s *seqState) removeWaiter(localPort uint16, target chan capture.ConnectionEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()

	waiters := s.waiters[localPort]
	if len(waiters) == 0 {
		return
	}

	dst := waiters[:0]
	for _, ch := range waiters {
		if ch != target {
			dst = append(dst, ch)
		}
	}

	if len(dst) == 0 {
		delete(s.waiters, localPort)
		return
	}
	s.waiters[localPort] = dst
}

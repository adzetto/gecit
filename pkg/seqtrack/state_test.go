package seqtrack

import (
	"testing"
	"time"

	"github.com/boratanrikulu/gecit/pkg/capture"
)

func TestSeqStateWait_ReturnsBufferedEvent(t *testing.T) {
	state := newSeqState()
	state.store(capture.ConnectionEvent{SrcPort: 44321, Seq: 10, Ack: 20})

	evt := state.wait(44321, 50*time.Millisecond)
	if evt == nil {
		t.Fatal("expected event, got nil")
	}
	if evt.Seq != 10 || evt.Ack != 20 {
		t.Fatalf("got seq/ack %d/%d, want 10/20", evt.Seq, evt.Ack)
	}
}

func TestSeqStateWait_WakesOnFutureEvent(t *testing.T) {
	state := newSeqState()

	go func() {
		time.Sleep(10 * time.Millisecond)
		state.store(capture.ConnectionEvent{SrcPort: 12345, Seq: 30, Ack: 40})
	}()

	evt := state.wait(12345, 200*time.Millisecond)
	if evt == nil {
		t.Fatal("expected event, got nil")
	}
	if evt.Seq != 30 || evt.Ack != 40 {
		t.Fatalf("got seq/ack %d/%d, want 30/40", evt.Seq, evt.Ack)
	}
}

func TestSeqStateWait_Timeout(t *testing.T) {
	state := newSeqState()

	evt := state.wait(55555, 20*time.Millisecond)
	if evt != nil {
		t.Fatalf("expected timeout nil, got %#v", evt)
	}
}

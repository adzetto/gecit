package seqtrack

import (
	"net"
	"time"

	"github.com/boratanrikulu/gecit/pkg/capture"
	"github.com/sirupsen/logrus"
)

type SeqTracker struct {
	detector capture.Detector
	state    *seqState
}

func NewSeqTracker(iface string, ports []uint16) (*SeqTracker, error) {
	det, err := capture.NewCapture(iface, ports)
	if err != nil {
		return nil, err
	}

	st := &SeqTracker{detector: det, state: newSeqState()}
	det.Start(func(evt capture.ConnectionEvent) {
		st.state.store(evt)
	})

	return st, nil
}

func (st *SeqTracker) WaitForSeqAck(localPort uint16, timeout time.Duration) *capture.ConnectionEvent {
	return st.state.wait(localPort, timeout)
}

func (st *SeqTracker) Stop() {
	if st.detector != nil {
		st.detector.Stop()
	}
}

var globalSeqTracker *SeqTracker

func SetSeqTracker(st *SeqTracker) {
	globalSeqTracker = st
}

func GetSeqAck(conn net.Conn) (seq, ack uint32) {
	if globalSeqTracker == nil {
		return 1, 1
	}

	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return 1, 1
	}

	localPort := uint16(tcpConn.LocalAddr().(*net.TCPAddr).Port)

	evt := globalSeqTracker.WaitForSeqAck(localPort, 500*time.Millisecond)
	if evt == nil {
		logrus.WithField("port", localPort).Warn("seq/ack fallback — Npcap may not be capturing")
		return 1, 1
	}

	return evt.Seq, evt.Ack
}

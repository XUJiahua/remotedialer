package remotedialer

import (
	"context"
	"fmt"
	"github.com/rancher/remotedialer/metrics"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type wsConn struct {
	sync.Mutex
	conn                         *websocket.Conn
	secondsElapsedAfterPongOrErr float64
}

func newWSConn(conn *websocket.Conn) *wsConn {
	w := &wsConn{
		conn:                         conn,
		secondsElapsedAfterPongOrErr: 0,
	}
	w.setupDeadline()
	return w
}

func (w *wsConn) WriteMessage(messageType int, deadline time.Time, data []byte) error {
	if deadline.IsZero() {
		w.Lock()
		defer w.Unlock()
		return w.conn.WriteMessage(messageType, data)
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		w.Lock()
		defer w.Unlock()
		done <- w.conn.WriteMessage(messageType, data)
	}()

	select {
	case <-ctx.Done():
		return fmt.Errorf("i/o timeout")
	case err := <-done:
		return err
	}
}

func (w *wsConn) NextReader() (int, io.Reader, error) {
	// reset read deadline for every read operation
	err := w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	if err != nil {
		return 0, nil, err
	}

	start := time.Now()
	// w.conn.NextReader will not return PING/PONG message type
	messageType, r, err := w.conn.NextReader()
	w.secondsElapsedAfterPongOrErr += time.Now().Sub(start).Seconds()
	if err != nil {
		w.observeSecondsElapsed()
	}

	metrics.IncTotalWebSocketMessageType(messageType)
	return messageType, r, err
}

func (w *wsConn) observeSecondsElapsed() {
	metrics.ObserveSecondsElapsedAfterPongOrErr(w.secondsElapsedAfterPongOrErr)
	// reset time escaped after receiving PONG
	w.secondsElapsedAfterPongOrErr = 0
}

func (w *wsConn) setupDeadline() {
	w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration))
	w.conn.SetPingHandler(func(string) error {
		w.Lock()
		err := w.conn.WriteControl(websocket.PongMessage, []byte(""), time.Now().Add(PingWaitDuration))
		w.Unlock()
		if err != nil {
			return err
		}
		if err := w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration)); err != nil {
			return err
		}
		return w.conn.SetWriteDeadline(time.Now().Add(PingWaitDuration))
	})
	w.conn.SetPongHandler(func(string) error {
		metrics.IncTotalWebSocketMessageType(websocket.PongMessage)
		w.observeSecondsElapsed()
		if err := w.conn.SetReadDeadline(time.Now().Add(PingWaitDuration)); err != nil {
			return err
		}
		return w.conn.SetWriteDeadline(time.Now().Add(PingWaitDuration))
	})

}

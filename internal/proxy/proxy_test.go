package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestIsConnectionClosedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "closed network connection", err: net.ErrClosed, want: true},
		{name: "closed pipe", err: io.ErrClosedPipe, want: true},
		{name: "broken pipe", err: syscall.EPIPE, want: true},
		{name: "connection reset", err: syscall.ECONNRESET, want: true},
		{name: "wrapped connection error", err: fmt.Errorf("copy data: %w", syscall.EPIPE), want: true},
		{name: "other error", err: errors.New("copy failed"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsConnectionClosedError(tt.err))
		})
	}
}

func TestRunContinuesAfterClosedConnectionError(t *testing.T) {
	t.Parallel()

	listener := newTestListener()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closedErrCh := make(chan error, 1)
	unexpectedErrCh := make(chan error, 1)
	var dialCount atomic.Int32

	p := &Proxy{
		Listener:   listener,
		RemoteAddr: "remote:80",
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			if dialCount.Add(1) == 1 {
				return readErrorConn{err: syscall.EPIPE}, nil
			}

			proxyConn, upstreamConn := net.Pipe()
			go func() {
				defer upstreamConn.Close()
				_, _ = upstreamConn.Write([]byte("ok"))
			}()
			return proxyConn, nil
		},
		OnError: func(err error) {
			if IsConnectionClosedError(err) {
				closedErrCh <- err
				return
			}
			unexpectedErrCh <- err
		},
	}

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- p.Run(ctx)
	}()

	firstConn := listener.connect()
	defer firstConn.Close()

	select {
	case connErr := <-closedErrCh:
		require.ErrorIs(t, connErr, syscall.EPIPE)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for closed connection error")
	}

	secondConn := listener.connect()
	defer secondConn.Close()
	require.NoError(t, secondConn.SetReadDeadline(time.Now().Add(time.Second)))

	got := make([]byte, 2)
	_, err := io.ReadFull(secondConn, got)
	require.NoError(t, err)
	require.Equal(t, "ok", string(got))

	select {
	case unexpectedErr := <-unexpectedErrCh:
		t.Fatalf("unexpected connection error: %v", unexpectedErr)
	default:
	}

	cancel()
	select {
	case runErr := <-runErrCh:
		require.NoError(t, runErr)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for proxy to stop")
	}
}

func TestRunReturnsListenerError(t *testing.T) {
	t.Parallel()

	listenerErr := errors.New("listener failed")
	listener := errorListener{err: listenerErr}

	p := &Proxy{Listener: listener}
	err := p.Run(context.Background())
	require.Error(t, err)
	require.ErrorContains(t, err, "accept local connection")
	require.ErrorIs(t, err, listenerErr)
}

type testListener struct {
	conns     chan net.Conn
	closed    chan struct{}
	closeOnce sync.Once
}

func newTestListener() *testListener {
	return &testListener{
		conns:  make(chan net.Conn),
		closed: make(chan struct{}),
	}
}

func (l *testListener) connect() net.Conn {
	clientConn, proxyConn := net.Pipe()
	l.conns <- proxyConn
	return clientConn
}

func (l *testListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *testListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.closed)
	})
	return nil
}

func (l *testListener) Addr() net.Addr {
	return &net.TCPAddr{}
}

type errorListener struct {
	err error
}

func (l errorListener) Accept() (net.Conn, error) { return nil, l.err }
func (errorListener) Close() error                { return nil }
func (errorListener) Addr() net.Addr              { return &net.TCPAddr{} }

// readErrorConn fails reads immediately so tests can deterministically exercise a proxy copy failure.
type readErrorConn struct {
	err error
}

func (c readErrorConn) Read([]byte) (int, error)       { return 0, c.err }
func (readErrorConn) Write(p []byte) (int, error)      { return len(p), nil }
func (readErrorConn) Close() error                     { return nil }
func (readErrorConn) LocalAddr() net.Addr              { return &net.TCPAddr{} }
func (readErrorConn) RemoteAddr() net.Addr             { return &net.TCPAddr{} }
func (readErrorConn) SetDeadline(time.Time) error      { return nil }
func (readErrorConn) SetReadDeadline(time.Time) error  { return nil }
func (readErrorConn) SetWriteDeadline(time.Time) error { return nil }

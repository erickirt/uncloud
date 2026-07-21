package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"syscall"
	"time"
)

// Proxy proxies local connections to a remote TCP address optionally using a custom dialer.
type Proxy struct {
	Listener    net.Listener
	RemoteAddr  string
	DialContext func(ctx context.Context, network, address string) (net.Conn, error)
	// OnError is called for errors that occur during proxying individual connections. It may be called concurrently
	// for different connections.
	OnError     func(error)
	activeConns sync.WaitGroup
}

// halfCloser is an interface for connections that support half-close.
type halfCloser interface {
	CloseWrite() error
}

// IsConnectionClosedError reports whether err indicates that a connection was closed or aborted by either peer.
// Callers can use it to ignore routine connection shutdown or broken pipe errors reported to Proxy.OnError.
func IsConnectionClosedError(err error) bool {
	return errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET)
}

// Run starts the proxy and runs until the context is canceled or the listener fails. It returns nil when the context
// is canceled. Errors handling individual connections are reported to OnError and do not stop the proxy.
func (p *Proxy) Run(ctx context.Context) error {
	if p.DialContext == nil {
		p.DialContext = (&net.Dialer{}).DialContext
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer p.Listener.Close()

	// Closing the listener unblocks Accept when the context is canceled. This works for both TCP and Unix listeners
	// and avoids polling with listener deadlines.
	stopClose := context.AfterFunc(ctx, func() {
		p.Listener.Close()
	})
	defer stopClose()

	var runErr error
	for {
		conn, err := p.Listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				break
			}

			runErr = fmt.Errorf("accept local connection: %w", err)
			cancel()
			break
		}

		p.activeConns.Add(1)
		go p.handleConnection(ctx, conn)
	}

	// Wait for all connections to finish.
	p.activeConns.Wait()
	return runErr
}

func (p *Proxy) handleConnection(ctx context.Context, localConn net.Conn) {
	defer p.activeConns.Done()
	defer localConn.Close()

	// Use a separate context with timeout for dialing the remote address.
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	remoteConn, err := p.DialContext(dialCtx, "tcp", p.RemoteAddr)
	if err != nil {
		if ctx.Err() == nil && p.OnError != nil {
			p.OnError(fmt.Errorf("connect remote address '%s': %w", p.RemoteAddr, err))
		}
		return
	}
	defer remoteConn.Close()

	// Closing both connections aborts both copies after cancellation or a copy error. A clean EOF still uses
	// half-close so the other direction can finish sending any remaining data.
	closeConnections := func() {
		localConn.Close()
		remoteConn.Close()
	}
	stopClose := context.AfterFunc(ctx, closeConnections)
	defer stopClose()

	done := make(chan error, 2)

	go func() {
		_, err := io.Copy(remoteConn, localConn)
		if err != nil {
			done <- err
			closeConnections()
			return
		}
		// Close write half of remote connection if supported.
		if hc, ok := remoteConn.(halfCloser); ok {
			hc.CloseWrite()
		}
		done <- nil
	}()

	go func() {
		_, err := io.Copy(localConn, remoteConn)
		if err != nil {
			done <- err
			closeConnections()
			return
		}
		// Close write half of local connection if supported.
		if hc, ok := localConn.(halfCloser); ok {
			hc.CloseWrite()
		}
		done <- nil
	}()

	// Wait for both copies to complete. The first error is the original failure because a copy reports it before
	// closing the connections to unblock the other copy.
	var copyErr error
	for range 2 {
		if err = <-done; err != nil && copyErr == nil {
			copyErr = err
		}
	}

	if copyErr != nil && ctx.Err() == nil && p.OnError != nil {
		p.OnError(fmt.Errorf("data copy: %w", copyErr))
	}
}

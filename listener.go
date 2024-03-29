// -----------------------------------------------------------------------------
// Copyright 2019 Timothy E. Peoples
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS
// IN THE SOFTWARE.
// -----------------------------------------------------------------------------

// Package peercred provides Listener - a net.Listener implementation leveraging
// the Linux SO_PEERCRED socket option to acquire the PID, UID, and GID of the
// foreign process connected to each socket. According to the socket(7) manual,
//
//	This is possible only for connected AF_UNIX stream
//	sockets and AF_UNIX stream and datagram socket pairs
//	created using socketpair(2).
//
// Therefore, peercred.Listener only supports Unix domain sockets and IP
// connections are not available.
//
// peercred.Listener is intended for use cases where a Unix domain server needs
// to reliably identify the process on the client side of each connection. By
// itself, peercred provides support for simple "unix" socket connections.
// Additional support for gRPC over Unix domain sockets is available with the
// subordinate package toolman.org/net/peercred/grpcpeer.
//
// A simple, unix-domain server can be written similar to the following:
//
//	// Create a new Listener listening on socketName
//	lsnr, err := peercred.Listen(ctx, socketName)
//	if err != nil {
//	    return err
//	}
//
//	// Wait for and accept an incoming connection
//	conn, err := lsnr.AcceptPeerCred()
//	if err != nil {
//	    return err
//	}
//
//	// conn.Ucred has fields Pid, Uid and Gid
//	fmt.Printf("Client PID=%d UID=%d\n", conn.Ucred.Pid, conn.Ucred.Uid)
//
// NOTE: Currently, this package only works on Linux.
// MacOS and FreeBSD are on the todo list. Windows isn't (nor are other OSs).
package peercred // import "toolman.org/net/peercred"

import (
	"context"
	"errors"
	"net"
	"sync"

	"golang.org/x/sys/unix"
)

// ErrAddrInUse is a convenience wrapper around the Posix errno value for
// EADDRINUSE.
const ErrAddrInUse = unix.EADDRINUSE

// Listener is an implementation of net.Listener that extracts peer credentials
// (i.e. PID, UID, GID) of the foreign process connected to each socket. Since
// the underlying features making this possible are only available for "unix"
// sockets, no "network" argument is required here ("unix" is implied). The
// acquired peer credentials are made available through the "Ucred" member of
// the *Conn returned by AcceptPeerCred.
//
// See 'SO_PEERCRED' in socket(7) for further details.
type Listener struct {
	once sync.Once
	net.Listener
}

// Listen returns a new *Listener listening on the Unix domain socket addr.
func Listen(ctx context.Context, addr string) (*Listener, error) {
	lc := new(net.ListenConfig)
	l, err := lc.Listen(ctx, "unix", addr)
	if err != nil {
		return nil, chkAddrInUseError(err)
	}

	return &Listener{Listener: l}, nil
}

// Close is a wrapper that calls the underlying net.Listener's Close method
// once and only once regardless how many times this method is called.
//
// Close contributes to implementing the net.Listener interface.
func (pcl *Listener) Close() error {
	var err error
	pcl.once.Do(func() {
		err = pcl.Listener.Close()
	})
	return err
}

// Accept is a convenience wrapper around AcceptPeerCred allowing callers
// utilizing the net.Listener interface to function as expected. The returned
// net.Conn is a *peercred.Conn that may be accessed with a type assertion.
// See AcceptPeerCred for details on possible error conditions.
//
// Accept contributes to implementing the  net.Listener interface.
func (pcl *Listener) Accept() (net.Conn, error) {
	switch conn, err := pcl.accept(context.Background()); err {
	case nil:
		return conn, nil
	default:
		return nil, err
	}
}

// AcceptPeerCred accepts a connection on the listening socket returning a *Conn
// containing the process credentials for the client. If the underlying Accept
// fails or if no peer process credentials can be extracted, AcceptPeerCred
// returns nil and an error.
func (pcl *Listener) AcceptPeerCred() (*Conn, error) {
	return pcl.accept(nil)
}

// AcceptContext behaves the same as AcceptPeerCred except it immediately
// returns nil and ctx.Err() if the Context is canceled. Note that the
// underlying Accept call is unblocked by closing the listening socket so,
// if this method is interrupted by a canceled Context, this *Listener will
// accept no new connections.
func (pcl *Listener) AcceptContext(ctx context.Context) (*Conn, error) {
	return pcl.accept(ctx)
}

func (pcl *Listener) accept(ctx context.Context) (*Conn, error) {
	if ctx != nil {
		// n.b. AcceptPeerCred calls this method with a nil Context so we should
		//      only do this stuff if ctx has a value.
		done := make(chan struct{})
		defer close(done)

		// A goroutine to close the listener if ctx gets cancelled before this
		// method returns (which closes the local 'done' channel via the above
		// 'defer'.
		go func() {
			select {
			case <-done:
			case <-ctx.Done():
				pcl.Close()
			}
		}()
	}

	conn, err := pcl.Listener.Accept()
	if err != nil {
		if ctx.Err() != nil {
			err = ctx.Err()
		}

		return nil, err
	}

	pcc := &Conn{Conn: conn}

	uc, ok := conn.(*net.UnixConn)
	if !ok {
		return pcc, nil
	}

	rc, err := uc.SyscallConn()
	if err != nil {
		return nil, err
	}

	var ucred *unix.Ucred
	cerr := rc.Control(func(fd uintptr) {
		ucred, err = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})

	if cerr != nil || err != nil {
		if err == nil {
			err = cerr
		}
		return nil, err
	}

	pcc.Ucred = ucred

	return pcc, nil
}

// Conn is a net.Conn containing the process credentials for the client
// side of a Unix domain socket connection.
type Conn struct {
	Ucred *unix.Ucred
	net.Conn
}

func asErrno(err error) unix.Errno {
	p := new(unix.Errno)
	if errors.As(err, p) {
		return *p
	}
	return 0
}

func chkAddrInUseError(err error) error {
	if errno := asErrno(err); errno == ErrAddrInUse {
		return errno
	}
	return err
}

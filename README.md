
# peercred [![Mit License][mit-img]][mit] [![GitHub Release][release-img]][release] [![GoDoc][godoc-img]][godoc] [![Go Report Card][reportcard-img]][reportcard] [![Build Status][travis-img]][travis]

`import "toolman.org/net/peercred"`

* [Install](#pkg-install)
* [Overview](#pkg-overview)
* [Index](#pkg-index)
* [Subdirectories](#pkg-subdirectories)

## <a name="pkg-install">Install</a>

```sh
    go get toolman.org/net/peercred
```

## <a name="pkg-overview">Overview</a>
Package peercred provides Listener - a net.Listener implementation leveraging
the Linux SO_PEERCRED socket option to acquire the PID, UID, and GID of the
foreign process connected to each socket. According to the socket(7) manual,

    This is possible only for connected AF_UNIX stream
    sockets and AF_UNIX stream and datagram socket pairs
    created using socketpair(2).

Therefore, peercred.Listener only supports Unix domain sockets and IP
connections are not available.

peercred.Listener is intended for use cases where a Unix domain server needs
to reliably identify the process on the client side of each connection. By
itself, peercred provides support for simple "unix" socket connections.
Additional support for gRPC over Unix domain sockets is available with the
subordinate package toolman.org/net/peercred/grpcpeer.

A simple, unix-domain server can be written similar to the following:

	// Create a new Listener listening on socketName
	lsnr, err := peercred.NewListener(ctx, socketName)
	if err != nil {
	    return err
	}
	
	// Wait for and accept an incoming connection
	conn, err := lsnr.AcceptPeerCred()
	if err != nil {
	    return err
	}
	
	// conn.Ucred has fields Pid, Uid and Gid
	fmt.Printf("Client PID=%d UID=%d\n", conn.Ucred.Pid, conn.Ucred.Uid)


## <a name="pkg-index">Index</a>
* [Constants](#pkg-constants)
* [type Conn](#Conn)
* [type Listener](#Listener)
  * [func Listen(ctx context.Context, addr string) (*Listener, error)](#NewListener)
  * [func (pcl *Listener) Accept() (net.Conn, error)](#Listener.Accept)
  * [func (pcl *Listener) AcceptPeerCred() (*Conn, error)](#Listener.AcceptPeerCred)


#### <a name="pkg-files">Package files</a>
[listener.go](/src/toolman.org/net/peercred/listener.go) 


## <a name="pkg-constants">Constants</a>
``` go
const ErrAddrInUse = unix.EADDRINUSE
```
ErrAddrInUse is a convenience wrapper around the Posix errno value for
EADDRINUSE.


## <a name="Conn">type</a> [Conn](/src/target/listener.go?s=4734:4791#L138)
``` go
type Conn struct {
    Ucred *unix.Ucred
    net.Conn
}

```
Conn is a net.Conn containing the process credentials for the client
side of a Unix domain socket connection.


## <a name="Listener">type</a> [Listener](/src/target/listener.go?s=2919:2965#L71)
``` go
type Listener struct {
    net.Listener
}

```
Listener is an implementation of net.Listener that extracts
the identity (i.e. pid, uid, gid) from the connection's client process.
This information is then made available through the Ucred member of
the *Conn returned by AcceptPeerCred or Accept (after a type
assertion).


### <a name="NewListener">func</a> [NewListener](/src/target/listener.go?s=3047:3116#L76)
``` go
func NewListener(ctx context.Context, addr string) (*Listener, error)
```
NewListener returns a new Listener listening on the Unix domain socket addr.


### <a name="Listener.Accept">func</a> (\*Listener) [Accept](/src/target/listener.go?s=3657:3712#L93)
``` go
func (pcl *Listener) Accept() (net.Conn, error)
```
Accept is a convenience wrapper around AcceptPeerCred allowing
Listener callers that utilize net.Listener to function
as expected. The returned net.Conn is a *Conn which may
be accessed through a type assertion. See AcceptPeerCred for
details on possible error conditions.

Accept contributes to implementing the  net.Listener interface.


### <a name="Listener.AcceptPeerCred">func</a> (\*Listener) [AcceptPeerCred](/src/target/listener.go?s=4020:4088#L101)
``` go
func (pcl *Listener) AcceptPeerCred() (*Conn, error)
```
AcceptPeerCred accepts a connection from the receiver's listener
returning a *Conn containing the process credentials for
the client. If the underlying Accept fails or if process credentials
cannot be extracted, AcceptPeerCred returns nil and an error.


[mit-img]: http://img.shields.io/badge/License-MIT-c41e3a.svg
[mit]: https://github.com/tep/net-peercred/blob/master/LICENSE

[release-img]: https://img.shields.io/github/release/tep/net-peercred/all.svg
[release]: https://github.com/tep/net-peercred/releases

[godoc-img]: https://godoc.org/toolman.org/net/peercred?status.svg
[godoc]: https://godoc.org/toolman.org/net/peercred

[reportcard-img]: https://goreportcard.com/badge/toolman.org/net/peercred
[reportcard]: https://goreportcard.com/report/toolman.org/net/peercred

[travis-img]: https://travis-ci.org/toolmanorg/net-peercred.svg?branch=master
[travis]: https://travis-ci.org/toolmanorg/net-peercred


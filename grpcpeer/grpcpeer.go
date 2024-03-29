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

// Package grpcpeer adds gRPC support to toolman.org/net/peercred with
// a ServerOption that helps gRPC recognize peercred Listeners and a helper
// function for extracting foreign process credentials from a service method's
// Context.
//
// The following example illustrates how to use a peercred.Listener with a
// gRPC server over a Unix domain socket:
//
//      // As with a simple unix-domain socket server, we'll first create
//      // a new peercred.Listener listening on socketName
//      lsnr, err := peercred.Listen(ctx, socketName)
//      if err != nil {
//          return err
//      }
//
//      // We'll need to tell gRPC how to deal with the process credentials
//      // acquired by the peercred Listener. This is easily accomplished by
//      // passing the grpcpeer.TransportCredentials ServerOption as we create
//      // the gRPC Server.
//      svr := grpc.NewServer(grpcpeer.TransportCredentials())
//
//      // Next, we'll install your service implementation into the gRPC
//      // Server we just created...
//      urpb.RegisterYourService(svr, svcImpl)
//
//      // ...and start the gRPC Server using the peercred.Listener created
//      // above.
//      svr.Serve(lsnr)
//
//  Finally, when you need to access the client's process credentials from
//  inside your service, pass the method's Context to grpcpeer.FromContext:
//
//      func (s *svcImpl) SomeMethod(ctx context.Context, req *SomeRequest, opts ...grpc.CallOption) (*SomeResponse, error) {
//          creds, err := grpcpeer.FromContext(ctx)
//          // (Unless there's an error) 'creds' now holds a *unix.Ucred
//          // containing the PID, UID and GID of the calling client process.
//      }
//
package grpcpeer

import (
	"context"
	"crypto/tls"
	"errors"
	"net"

	"golang.org/x/sys/unix"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"

	"toolman.org/net/peercred"
)

// ErrNoPeer is returned by FromContext if the provided Context contains
// no gRPC peer.
var ErrNoPeer = errors.New("context has no grpc peer")

// ErrNoCredentials is returned by FromContext if the provided Context
// contains no peer process credentials.
var ErrNoCredentials = errors.New("context contains no credentials")

var errNotImplemented = errors.New("not implemented")

// TransportCredentials returns a grpc.ServerOption that exposes the peer
// process credentials (i.e. PID, UID, GID) extracted by a peercred Listener.
// The peer credentials are available by passing a server method's Context
// to this package's FromContext function.
func TransportCredentials() grpc.ServerOption {
	return grpc.Creds(&peerCredentials{})
}

// TLSTransportCredentials is similar to TransportCredentials except that
// accepts a *tls.Config for use by the gRPC server.
func TLSTransportCredentials(cfg *tls.Config) grpc.ServerOption {
	return grpc.Creds(&peerCredentials{credentials.NewTLS(cfg)})
}

// TLSClientCredentials returns a grpc.DialOption which uses the provided
// *tls.Config as the client's certificate when dialing a similarly configured
// gRPC server.
func TLSClientCredentials(cfg *tls.Config) grpc.DialOption {
	if cfg == nil {
		cfg = new(tls.Config)
	}

	return grpc.WithTransportCredentials(&peerCredentials{credentials.NewTLS(cfg)})
}

// peerCredentials implements the gRPC TransportCredentials interface.
type peerCredentials struct {
	tcreds credentials.TransportCredentials
}

// ClientHandshake contributes to the implementation of the TransportCredentials
// interface from package google.golang.org/grpc/credentials.
func (pc *peerCredentials) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	if pc != nil && pc.tcreds != nil {
		return pc.tcreds.ClientHandshake(ctx, authority, conn)
	}
	return nil, nil, errNotImplemented
}

// ServerHandshake contributes to the implementation of the TransportCredentials
// interface from package google.golang.org/grpc/credentials.
func (pc *peerCredentials) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	ci := new(credInfo)
	// First, capture Ucred from conn (if possible)
	if pcConn, ok := conn.(*peercred.Conn); ok {
		ci.ucred = pcConn.Ucred
	}

	// If we have no underlying TransportCredentials, we're done.
	if pc.tcreds == nil {
		return conn, ci, nil
	}

	// Now, call the real ServerHandshake...
	tlsConn, info, err := pc.tcreds.ServerHandshake(conn)
	if err != nil {
		return nil, nil, err
	}

	// ...and merge the results.
	ci.AuthInfo = info

	return tlsConn, ci, nil
}

// Info contributes to the implementation of the TransportCredentials interface
// from package google.golang.org/grpc/credentials.
func (pc *peerCredentials) Info() credentials.ProtocolInfo {
	if pc.tcreds != nil {
		return pc.tcreds.Info()
	}

	// NOTE: There's little to no documentation on what this struct
	//       should contain but, after a hasty perusal of the code,
	//       it appears that setting SecurityProtocol to a value
	//       unbeknownst to others should be enough to keep gRPC's
	//       guts from trying to initiate a TLS negotiation.
	return credentials.ProtocolInfo{
		SecurityProtocol: "peer",
	}
}

// Clone contributes to the implementation of the TransportCredentials interface
// from package google.golang.org/grpc/credentials.
func (pc *peerCredentials) Clone() credentials.TransportCredentials {
	c := *pc
	return &c
}

// OverrideServerName contributes to the implementation of the TransportCredentials
// interface from package google.golang.org/grpc/credentials.
func (pc *peerCredentials) OverrideServerName(s string) error {
	if pc == nil || pc.tcreds == nil {
		return nil
	}

	return pc.tcreds.OverrideServerName(s)
}

// credInfo is a wrapper around the Ucred struct from golang.org/x/sys/unix
// allowing it to be used as the AuthInfo member of a gRPC peer.
//
// This is part of the mechanism used for plumbing *Ucred values through
// the gRPC framework and is not intended for general use.
type credInfo struct {
	ucred *unix.Ucred
	credentials.AuthInfo
}

// AuthType implements the grpc/credentials AuthInfo interface to enable
// plumbing *unix.Ucred values through the gRPC framework.
func (ci *credInfo) AuthType() string {
	if ci == nil {
		return ""
	}

	if ci.AuthInfo == nil {
		return "PeerCred"
	}

	return ci.AuthInfo.AuthType()
}

// FromContext extracts peer process credentials, if any, from the given
// Context. This is only possible if the gRPC server was creating with the
// ServerOption provided by this package's TransportCredentials function.
//
// If the provided Context has no gRPC peer, ErrNoPeer is returned. If the
// Context's peer is of the wrong type (i.e. contains no peer process
// credentials), ErrNoCredentials will be returned.
func FromContext(ctx context.Context) (*unix.Ucred, error) {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return nil, ErrNoPeer
	}

	if ci, ok := p.AuthInfo.(*credInfo); ok {
		return ci.ucred, nil
	}

	return nil, ErrNoCredentials
}

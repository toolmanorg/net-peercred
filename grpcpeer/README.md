
# grpcpeer
`import "toolman.org/net/peercred/grpcpeer"`

* [Install](#pkg-install)
* [Overview](#pkg-overview)
* [Index](#pkg-index)

## <a name="pkg-install">Install</a>

```sh
    go get toolman.org/net/peercred/grpcpeer
```

## <a name="pkg-overview">Overview</a>
Package grpcpeer adds gRPC support to toolman.org/net/peercred with
a ServerOption that helps gRPC recognize peercred Listeners and a helper
function for extracting foreign process credentials from a service method's
Context.

The following example illustrates how to use a peercred.Listener with a
gRPC server over a Unix domain socket:


	    // As with a simple unix-domain socket server, we'll first create
	    // a new peercred.Listener listening on socketName
	    lsnr, err := peercred.NewListener(ctx, socketName)
	    if err != nil {
	        return err
	    }
	
	    // We'll need to tell gRPC how to deal with the process credentials
	    // acquired by the peercred Listener. This is easily accomplished by
	    // passing this package's TransportCredentials ServerOption as we
	    // create the gRPC Server.
	    svr := grpc.NewServer(grpcpeer.TransportCredentials())
	
	    // Next, we'll install your service implementation into the gRPC
	    // Server we just created...
	    urpb.RegisterYourService(svr, svcImpl)
	
	    // ...and start the gRPC Server using the peercred.Listener created
	    // above.
	    svr.Serve(lsnr)
	
	Finally, when you need to access the client's process creds from one of
	your service's methods, pass the method's Context to this package's
	FromContext function.
	
	    func (s *svcImpl) SomeMethod(ctx context.Context, req *SomeRequest, opts ...grpc.CallOption) (*SomeResponse, error) {
	        creds, err := grpcpeer.FromContext(ctx)
	        // (Unless there's an error) creds now holds a *unix.Ucred
	        // containing the PID, UID and GID of the calling client process.
	    }




## <a name="pkg-index">Index</a>
* [Variables](#pkg-variables)
* [func FromContext(ctx context.Context) (*unix.Ucred, error)](#FromContext)
* [func TransportCredentials() grpc.ServerOption](#TransportCredentials)


#### <a name="pkg-files">Package files</a>
[creds.go](/src/toolman.org/net/peercred/grpcpeer/creds.go) 


## <a name="pkg-variables">Variables</a>
``` go
var ErrNoCredentials = errors.New("context contains no credentials")
```
ErrNoCredentials is returned by FromContext if the provided Context
contains no peer process credentials.

``` go
var ErrNoPeer = errors.New("context has no grpc peer")
```
ErrNoPeer is returned by FromContext if the provided Context contains
no gRPC peer.


## <a name="TransportCredentials">func</a> [TransportCredentials](/src/target/creds.go?s=3701:3746#L89)
``` go
func TransportCredentials() grpc.ServerOption
```
TransportCredentials returns a grpc.ServerOption that exposes the peer
process credentials (i.e. PID, UID, GID) extracted by a peercred Listener.
The peer credentials are available by passing a server method's Context
to this package's FromContext function.


## <a name="FromContext">func</a> [FromContext](/src/target/creds.go?s=5662:5720#L141)
``` go
func FromContext(ctx context.Context) (*unix.Ucred, error)
```
FromContext extracts peer process credentials, if any, from the given
Context. This is only possible if the gRPC server was creating with the
ServerOption provided by this package's TransportCredentials function.

If the provided Context has no gRPC peer, ErrNoPeer is returned. If the
Context's peer is of the wrong type (i.e. contains no peer process
credentials), ErrNoCredentials will be returned.

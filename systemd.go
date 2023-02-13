// Copyright © 2023 Timothy E. Peoples

package peercred

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const fdBase = 3

// SDListen returns a new *Listener for the systemd activated socket provided
// to the current process.  If systemd provides more than one socket, SDListen
// will close them all and return an error.
// Use SDListenNames if you need to acquire multiple Listeners.
func SDListen() (*Listener, error) {
	lismap, err := SDListenNames()
	if err != nil {
		return nil, err
	}

	switch len(lismap) {
	case 0:
		return nil, errors.New("found no systemd activated sockets")

	case 1:
		for _, lis := range lismap {
			return lis, nil
		}
	}

	for _, lis := range lismap {
		lis.Close()
	}
	return nil, errors.New("found multiple systemd activated sockets")
}

// SDListenNames returns a map of SocketName -> *Listener for each systemd
// activated socket provided to the current process.
//
// The SocketName map key is defined by the FileDescriptorName directive
// in the associated socket's systemd unit -- or, if unspecified, the value
// defaults to the name of the socket unit, including its '.socket' suffix.
func SDListenNames() (map[string]*Listener, error) {
	lpid, lfds, lfdnames := os.Getenv("LISTEN_PID"), os.Getenv("LISTEN_FDS"), strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")
	if lpid == "" || lfds == "" {
		return nil, errors.New("systemd socket not found")
	}

	pid := os.Getpid()

	if i, err := strconv.Atoi(lpid); err != nil || i != pid {
		if err == nil {
			err = fmt.Errorf("systemd socket pid mismatch: got %d; wanted %d", i, pid)
		}
		return nil, err
	}

	fdcnt, err := strconv.Atoi(lfds)
	if err != nil {
		return nil, err
	}

	if ncnt := len(lfdnames); ncnt != fdcnt {
		fmt.Errorf("systemd socket count mismatch: got %d; wanted %d", fdcnt, ncnt)
	}

	out := make(map[string]*Listener)

	for i := 0; i < fdcnt; i++ {
		n := lfdnames[i]
		lis, err := net.FileListener(os.NewFile(uintptr(fdBase+i), n))
		if err != nil {
			return nil, err
		}
		out[n] = &Listener{Listener: lis}
	}

	return out, nil
}

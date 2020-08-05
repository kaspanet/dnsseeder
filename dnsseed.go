// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/kaspanet/kaspad/config"
	"github.com/kaspanet/kaspad/netadapter/netadaptermock"
	"github.com/kaspanet/kaspad/protocol/common"
	"net"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"

	"github.com/kaspanet/dnsseeder/version"
	"github.com/kaspanet/kaspad/dnsseed"
	"github.com/kaspanet/kaspad/util/panics"
	"github.com/kaspanet/kaspad/util/profiling"

	"github.com/kaspanet/kaspad/signal"
	"github.com/kaspanet/kaspad/wire"

	_ "net/http/pprof"
)

const (
	// nodeTimeout defines the timeout time waiting for
	// a response from a node.
	nodeTimeout = time.Second * 3

	// requiredServices describes the default services that are
	// required to be supported by outbound peers.
	requiredServices = wire.SFNodeNetwork
)

var (
	amgr             *Manager
	wg               sync.WaitGroup
	peersDefaultPort int
	systemShutdown   int32
	defaultSeeder    *wire.NetAddress
)

// hostLookup returns the correct DNS lookup function to use depending on the
// passed host and configuration options. For example, .onion addresses will be
// resolved using the onion specific proxy if one was specified, but will
// otherwise treat the normal proxy as tor unless --noonion was specified in
// which case the lookup will fail. Meanwhile, normal IP addresses will be
// resolved using tor if a proxy was specified unless --noonion was also
// specified in which case the normal system DNS resolver will be used.
func hostLookup(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}

func creep() {
	defer wg.Done()

	netAdapter, err := netadaptermock.New(&config.Config{})
	if err != nil {
		log.Errorf("Could not start net adapter")
		return
	}

	var wgCreep sync.WaitGroup
	for {
		peers := amgr.Addresses()
		if len(peers) == 0 && amgr.AddressCount() == 0 {
			// Add peers discovered through DNS to the address manager.
			dnsseed.SeedFromDNS(ActiveConfig().NetParams(), "", requiredServices, true,
				nil, hostLookup, func(addrs []*wire.NetAddress) {
					amgr.AddAddresses(addrs)
				})
			peers = amgr.Addresses()
		}
		if len(peers) == 0 {
			log.Infof("No stale addresses -- sleeping for 10 minutes")
			for i := 0; i < 600; i++ {
				time.Sleep(time.Second)
				if atomic.LoadInt32(&systemShutdown) != 0 {
					log.Infof("Creep thread shutdown")
					return
				}
			}
			continue
		}

		for _, addr := range peers {
			if atomic.LoadInt32(&systemShutdown) != 0 {
				log.Infof("Waiting creep threads to terminate")
				wgCreep.Wait()
				log.Infof("Creep thread shutdown")
				return
			}
			wgCreep.Add(1)
			go func(addr *wire.NetAddress) {
				defer wgCreep.Done()

				err := pollPeer(netAdapter, addr)
				if err != nil {
					log.Warnf(err.Error())
					if defaultSeeder != nil && addr == defaultSeeder {
						panics.Exit(log, "failed to poll default seeder")
					}
				}
			}(addr)
		}
		wgCreep.Wait()
	}
}

func pollPeer(netAdapter *netadaptermock.NetAdapterMock, addr *wire.NetAddress) error {
	peerAddress := net.JoinHostPort(addr.IP.String(), strconv.Itoa(int(addr.Port)))

	routes, err := netAdapter.Connect(peerAddress)
	if err != nil {
		return errors.Wrapf(err, "could not connect to %s", peerAddress)
	}
	defer routes.Close()

	msgRequestAddresses := wire.NewMsgRequestAddresses(true, nil)
	err = routes.OutgoingRoute.Enqueue(msgRequestAddresses)
	if err != nil {
		return errors.Wrapf(err, "failed to request addresses from %s", peerAddress)
	}

	message, err := routes.WaitForMessageOfType(wire.CmdAddresses, common.DefaultTimeout)
	if err != nil {
		return errors.Wrapf(err, "failed to receive addresses from %s", peerAddress)
	}
	msgAddresses := message.(*wire.MsgAddresses)

	added := amgr.AddAddresses(msgAddresses.AddrList)
	log.Infof("Peer %s sent %d addresses, %d new",
		peerAddress, len(msgAddresses.AddrList), added)

	return nil
}

func main() {
	defer panics.HandlePanic(log, "main", nil)
	interrupt := signal.InterruptListener()

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}

	// Show version at startup.
	log.Infof("Version %s", version.Version())

	// Enable http profiling server if requested.
	if cfg.Profile != "" {
		profiling.Start(cfg.Profile, log)
	}

	amgr, err = NewManager(defaultHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	peersDefaultPort, err = strconv.Atoi(ActiveConfig().NetParams().DefaultPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid peers default port %s: %v\n", ActiveConfig().NetParams().DefaultPort, err)
		os.Exit(1)
	}

	if len(cfg.Seeder) != 0 {
		ip := net.ParseIP(cfg.Seeder)
		if ip == nil {
			hostAddrs, err := net.LookupHost(cfg.Seeder)
			if err != nil {
				log.Warnf("Failed to resolve seed host: %v, %v, ignoring", cfg.Seeder, err)
			} else {
				ip = net.ParseIP(hostAddrs[0])
				if ip == nil {
					log.Warnf("Failed to resolve seed host: %v, ignoring", cfg.Seeder)
				}
			}
		}
		if ip != nil {
			defaultSeeder = wire.NewNetAddressIPPort(ip, uint16(peersDefaultPort), requiredServices)
			amgr.AddAddresses([]*wire.NetAddress{defaultSeeder})
		}
	}

	wg.Add(1)
	spawn("main-creep", creep)

	dnsServer := NewDNSServer(cfg.Host, cfg.Nameserver, cfg.Listen)
	wg.Add(1)
	spawn("main-DNSServer.Start", dnsServer.Start)

	defer func() {
		log.Infof("Gracefully shutting down the seeder...")
		atomic.StoreInt32(&systemShutdown, 1)
		close(amgr.quit)
		wg.Wait()
		amgr.wg.Wait()
		log.Infof("Seeder shutdown complete")
	}()

	// Wait until the interrupt signal is received from an OS signal or
	// shutdown is requested through one of the subsystems such as the RPC
	// server.
	<-interrupt
}

// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/daglabs/btcd/connmgr"
	"github.com/daglabs/btcd/peer"
	"github.com/daglabs/btcd/wire"
)

const (
	// defaultAddressTimeout defines the duration to wait
	// for new addresses.
	defaultAddressTimeout = time.Minute * 10

	// defaultNodeTimeout defines the timeout time waiting for
	// a response from a node.
	defaultNodeTimeout = time.Second * 3

	// defaultRequiredServices describes the default services that are
	// required to be supported by outbound peers.
	defaultRequiredServices = wire.SFNodeNetwork
)

var (
	amgr             *Manager
	wg               sync.WaitGroup
	peersDefaultPort int
)

// hostLookup returns the correct DNS lookup function to use depending on the
// passed host and configuration options.  For example, .onion addresses will be
// resolved using the onion specific proxy if one was specified, but will
// otherwise treat the normal proxy as tor unless --noonion was specified in
// which case the lookup will fail.  Meanwhile, normal IP addresses will be
// resolved using tor if a proxy was specified unless --noonion was also
// specified in which case the normal system DNS resolver will be used.
func hostLookup(host string) ([]net.IP, error) {
	return net.LookupIP(host)
}

func creep() {
	defer wg.Done()

	onaddr := make(chan struct{})
	onversion := make(chan struct{})
	config := peer.Config{
		UserAgentName:    "daglabs-sniffer",
		UserAgentVersion: "0.0.1",
		DAGParams:        activeNetParams,
		DisableRelayTx:   true,

		Listeners: peer.MessageListeners{
			OnAddr: func(p *peer.Peer, msg *wire.MsgAddr) {
				added := amgr.AddAddresses(msg.AddrList)
				log.Printf("Peer %v sent %v addresses, %d new",
					p.Addr(), len(msg.AddrList), added)
				onaddr <- struct{}{}
			},
			OnVersion: func(p *peer.Peer, msg *wire.MsgVersion) {
				log.Printf("Adding peer %v with services %v",
					p.NA().IP.String(), p.Services())
				// notify that version is received and Peer's subnetwork ID is updated
				onversion <- struct{}{}
			},
		},
		SubnetworkID:  &wire.SubnetworkIDSupportsAll,
		DnsSeederPeer: true,
	}

	var wg sync.WaitGroup
	for {
		peers := amgr.Addresses()
		if len(peers) == 0 && amgr.AddressCount() == 0 {
			// Add peers discovered through DNS to the address manager.
			connmgr.SeedFromDNS(activeNetParams, defaultRequiredServices, &wire.SubnetworkIDSupportsAll, hostLookup, func(addrs []*wire.NetAddress) {
				amgr.AddAddresses(addrs)
			})
			peers = amgr.Addresses()
		}
		if len(peers) == 0 {
			log.Printf("No stale addresses -- sleeping for %v",
				defaultAddressTimeout)
			time.Sleep(defaultAddressTimeout)
			continue
		}

		wg.Add(len(peers))

		for _, addr := range peers {
			go func(addr *wire.NetAddress) {
				defer wg.Done()

				host := net.JoinHostPort(addr.IP.String(), fmt.Sprintf("%d", int(addr.Port)))
				p, err := peer.NewOutboundPeer(&config, host)
				if err != nil {
					log.Printf("NewOutboundPeer on %v: %v",
						host, err)
					return
				}
				amgr.Attempt(addr.IP)
				conn, err := net.DialTimeout("tcp", p.Addr(),
					defaultNodeTimeout)
				if err != nil {
					log.Printf("%v", err)
					return
				}
				p.AssociateConnection(conn)

				// Wait version messsage or timeout in case of failure.
				select {
				case <-onversion:
					// Mark this peer as a good node.
					amgr.Good(p.NA().IP, p.Services(), p.SubnetworkID())
					// Ask peer for some addresses.
					p.QueueMessage(wire.NewMsgGetAddr(), nil)
				case <-time.After(defaultNodeTimeout):
					log.Printf("version timeout on peer %v",
						p.Addr())
					p.Disconnect()
					return
				}

				select {
				case <-onaddr:
				case <-time.After(defaultNodeTimeout):
					log.Printf("getaddr timeout on peer %v",
						p.Addr())
					p.Disconnect()
					return
				}
				p.Disconnect()
			}(addr)
		}
		wg.Wait()
		log.Printf("Sleeping for %v", defaultAddressTimeout)
		time.Sleep(defaultAddressTimeout)
	}
}

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig: %v\n", err)
		os.Exit(1)
	}
	amgr, err = NewManager(defaultHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	peersDefaultPort, _ := strconv.Atoi(activeNetParams.DefaultPort)

	if len(cfg.Seeder) != 0 {
		ip := net.ParseIP(cfg.Seeder)
		if ip == nil {
			hostAdrs, err := net.LookupHost(cfg.Seeder)
			if err != nil {
				log.Printf("Failed to resolve seed host: %v, %v, ignoring", cfg.Seeder, err)
			} else {
				ip = net.ParseIP(hostAdrs[0])
				if ip == nil {
					log.Printf("Failed to resolve seed host: %v, ignoring", cfg.Seeder)
				}
			}
		}
		if ip != nil {
			amgr.AddAddresses([]*wire.NetAddress{wire.NewNetAddressIPPort(ip, uint16(peersDefaultPort), defaultRequiredServices)})
		}
	}

	wg.Add(1)
	go creep()

	dnsServer := NewDNSServer(cfg.Host, cfg.Nameserver, cfg.Listen)
	go dnsServer.Start()

	wg.Wait()
}

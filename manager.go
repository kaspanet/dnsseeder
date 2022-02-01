// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/kaspanet/kaspad/infrastructure/network/addressmanager"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/miekg/dns"
	"github.com/pkg/errors"
)

// Node repesents a node in the Kaspa network
type Node struct {
	Addr         *appmessage.NetAddress
	LastAttempt  time.Time
	LastSuccess  time.Time
	LastSeen     time.Time
	SubnetworkID *externalapi.DomainSubnetworkID
}

// Manager is dnsseeder's main worker-type, storing all information required
// for operation
type Manager struct {
	mtx sync.RWMutex

	nodes     map[string]*Node
	wg        sync.WaitGroup
	quit      chan struct{}
	peersFile string
}

const (
	// defaultMaxAddresses is the maximum number of addresses to return.
	defaultMaxAddresses = 16

	// defaultStaleTimeout is the time in which a host is considered
	// stale.
	defaultStaleTimeout = time.Hour

	// dumpAddressInterval is the interval used to dump the address
	// cache to disk for future use.
	dumpAddressInterval = time.Second * 30

	// peersFilename is the name of the file.
	peersFilename = "nodes.json"

	// pruneAddressInterval is the interval used to run the address
	// pruner.
	pruneAddressInterval = time.Minute * 1

	// pruneExpireTimeout is the expire time in which a node is
	// considered dead.
	pruneExpireTimeout = time.Hour * 8
)

// NewManager constructs and returns a new dnsseeder manager, with the provided dataDir
func NewManager(dataDir string) (*Manager, error) {
	amgr := Manager{
		nodes:     make(map[string]*Node),
		peersFile: filepath.Join(dataDir, peersFilename),
		quit:      make(chan struct{}),
	}

	err := amgr.deserializePeers()
	if err != nil {
		log.Warnf("Failed to parse file %s: %v", amgr.peersFile, err)
		// if it is invalid we nuke the old one unconditionally.
		err = os.Remove(amgr.peersFile)
		if err != nil {
			log.Warnf("Failed to remove corrupt peers file %s: %v",
				amgr.peersFile, err)
		}
	}

	amgr.wg.Add(1)
	spawn("NewManager-Manager.addressHandler", amgr.addressHandler)

	return &amgr, nil
}

// AddAddresses adds an address to this dnsseeder manager, and returns the number of
// address currently held
func (m *Manager) AddAddresses(addrs []*appmessage.NetAddress) int {
	var count int

	m.mtx.Lock()
	for _, addr := range addrs {
		if !addressmanager.IsRoutable(addr, ActiveConfig().NetParams().AcceptUnroutable) {
			continue
		}
		addrStr := addr.IP.String()

		_, exists := m.nodes[addrStr]
		if exists {
			m.nodes[addrStr].LastSeen = time.Now()
			continue
		}
		node := Node{
			Addr:     addr,
			LastSeen: time.Now(),
		}
		m.nodes[addrStr] = &node
		count++
	}
	m.mtx.Unlock()

	return count
}

// Addresses returns IPs that need to be tested again.
func (m *Manager) Addresses() []*appmessage.NetAddress {
	addrs := make([]*appmessage.NetAddress, 0, defaultMaxAddresses*8)
	now := time.Now()
	i := defaultMaxAddresses

	m.mtx.RLock()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}
		if now.Sub(node.LastSuccess) < defaultStaleTimeout ||
			now.Sub(node.LastAttempt) < defaultStaleTimeout {
			continue
		}
		addrs = append(addrs, node.Addr)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

// AddressCount returns number of known nodes.
func (m *Manager) AddressCount() int {
	return len(m.nodes)
}

// GoodAddresses returns good working IPs that match both the
// passed DNS query type and have the requested services.
func (m *Manager) GoodAddresses(qtype uint16, includeAllSubnetworks bool, subnetworkID *externalapi.DomainSubnetworkID,
) []*appmessage.NetAddress {
	addrs := make([]*appmessage.NetAddress, 0, defaultMaxAddresses)
	i := defaultMaxAddresses

	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return addrs
	}

	now := time.Now()
	m.mtx.RLock()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}

		if node.Addr.Port != uint16(peersDefaultPort) {
			continue
		}

		if !includeAllSubnetworks && !node.SubnetworkID.Equal(subnetworkID) {
			continue
		}

		if qtype == dns.TypeA && node.Addr.IP.To4() == nil {
			continue
		} else if qtype == dns.TypeAAAA && node.Addr.IP.To4() != nil {
			continue
		}

		if node.LastSuccess.IsZero() ||
			now.Sub(node.LastSuccess) > defaultStaleTimeout {
			continue
		}

		addrs = append(addrs, node.Addr)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

// Attempt updates the last connection attempt for the specified ip address to now
func (m *Manager) Attempt(ip net.IP) {
	m.mtx.Lock()
	node, exists := m.nodes[ip.String()]
	if exists {
		node.LastAttempt = time.Now()
	}
	m.mtx.Unlock()
}

// Good updates the last successful connection attempt for the specified ip address to now
func (m *Manager) Good(ip net.IP, subnetworkid *externalapi.DomainSubnetworkID) {
	m.mtx.Lock()
	node, exists := m.nodes[ip.String()]
	if exists {
		node.LastSuccess = time.Now()
		node.SubnetworkID = subnetworkid
	}
	m.mtx.Unlock()
}

// addressHandler is the main handler for the address manager. It must be run
// as a goroutine.
func (m *Manager) addressHandler() {
	defer m.wg.Done()
	pruneAddressTicker := time.NewTicker(pruneAddressInterval)
	defer pruneAddressTicker.Stop()
	dumpAddressTicker := time.NewTicker(dumpAddressInterval)
	defer dumpAddressTicker.Stop()
out:
	for {
		select {
		case <-dumpAddressTicker.C:
			m.savePeers()
		case <-pruneAddressTicker.C:
			m.prunePeers()
		case <-m.quit:
			break out
		}
	}
	log.Infof("Address manager: saving peers")
	m.savePeers()
	log.Infof("Address manager shoutdown")
}

func (m *Manager) prunePeers() {
	var count int
	now := time.Now()
	m.mtx.Lock()

	lastSeenAbovePruneExpire := func(node *Node) bool {
		return now.Sub(node.LastSeen) > pruneExpireTimeout
	}
	hadAttemptsButNoSuccess := func(node *Node) bool {
		return !node.LastAttempt.IsZero() && node.LastSuccess.IsZero()
	}
	hadSuccessButLongTimeAgo := func(node *Node) bool {
		return !node.LastSuccess.IsZero() && now.Sub(node.LastSuccess) > pruneExpireTimeout
	}

	for k, node := range m.nodes {
		if lastSeenAbovePruneExpire(node) ||
			hadAttemptsButNoSuccess(node) ||
			hadSuccessButLongTimeAgo(node) {

			delete(m.nodes, k)
			count++
		}
	}
	l := len(m.nodes)
	m.mtx.Unlock()

	log.Infof("Pruned %d addresses: %d remaining", count, l)
}

func (m *Manager) deserializePeers() error {
	filePath := m.peersFile
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	r, err := os.Open(filePath)
	if err != nil {
		return errors.Errorf("%s error opening file: %v", filePath, err)
	}
	defer r.Close()

	var nodes map[string]*Node
	dec := json.NewDecoder(r)
	err = dec.Decode(&nodes)
	if err != nil {
		return errors.Errorf("error reading %s: %v", filePath, err)
	}

	l := len(nodes)

	m.mtx.Lock()
	m.nodes = nodes
	m.mtx.Unlock()

	log.Infof("%d nodes loaded", l)
	return nil
}

func (m *Manager) savePeers() {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	// Write temporary peers file and then move it into place.
	tmpfile := m.peersFile + ".new"
	w, err := os.Create(tmpfile)
	if err != nil {
		log.Errorf("Error opening file %s: %v", tmpfile, err)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(&m.nodes); err != nil {
		log.Errorf("Failed to encode file %s: %v", tmpfile, err)
		return
	}
	if err := w.Close(); err != nil {
		log.Errorf("Error closing file %s: %v", tmpfile, err)
		return
	}
	if err := os.Rename(tmpfile, m.peersFile); err != nil {
		log.Errorf("Error writing file %s: %v", m.peersFile, err)
		return
	}
}

// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/decred/dcrd/wire"
	"github.com/miekg/dns"
)

type Node struct {
	IP          net.IP
	Services    wire.ServiceFlag
	LastAttempt time.Time
	LastSuccess time.Time
}

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
)

func NewManager(dataDir string) (*Manager, error) {
	amgr := Manager{
		nodes:     make(map[string]*Node),
		peersFile: filepath.Join(dataDir, peersFilename),
		quit:      make(chan struct{}),
	}

	err := amgr.deserializePeers()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to parse file %s: %v", amgr.peersFile, err)
		// if it is invalid we nuke the old one unconditionally.
		err = os.Remove(amgr.peersFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove corrupt peers file %s: %v",
				amgr.peersFile, err)
		}
	}

	go amgr.addressHandler()

	return &amgr, nil
}

func (m *Manager) AddAddresses(addrs []net.IP) {
	m.mtx.Lock()
	for _, addr := range addrs {
		addrStr := addr.String()

		_, exists := m.nodes[addrStr]
		if exists {
			continue
		}
		node := Node{
			IP: addr,
		}
		m.nodes[addrStr] = &node
	}
	m.mtx.Unlock()
}

// Addresses returns IPs that need to be tested again.
func (m *Manager) Addresses() []net.IP {
	addrs := make([]net.IP, 0, defaultMaxAddresses)
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
		addrs = append(addrs, node.IP)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

// GoodAddresses returns good working IPs that match both the
// passed DNS query type and have the requested services.
func (m *Manager) GoodAddresses(qtype uint16, services wire.ServiceFlag) []net.IP {
	addrs := make([]net.IP, 0, defaultMaxAddresses)
	i := defaultMaxAddresses

	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return addrs
	}

	m.mtx.RLock()
	for _, node := range m.nodes {
		if i == 0 {
			break
		}

		if qtype == dns.TypeA && node.IP.To4() == nil {
			continue
		} else if qtype == dns.TypeAAAA && node.IP.To4() != nil {
			continue
		}

		if node.LastSuccess.IsZero() ||
			time.Since(node.LastSuccess) > defaultStaleTimeout {
			continue
		}
		if node.Services&services != services {
			continue
		}
		addrs = append(addrs, node.IP)
		i--
	}
	m.mtx.RUnlock()

	return addrs
}

func (m *Manager) Attempt(ip net.IP) {
	m.mtx.Lock()
	node, exists := m.nodes[ip.String()]
	if !exists {
		m.mtx.Unlock()
		return
	}
	node.LastAttempt = time.Now()
	m.mtx.Unlock()
}

func (m *Manager) Good(ip net.IP, services wire.ServiceFlag) {
	m.mtx.Lock()
	node, exists := m.nodes[ip.String()]
	if !exists {
		m.mtx.Unlock()
		return
	}
	node.Services = services
	node.LastSuccess = time.Now()
	m.mtx.Unlock()
}

// addressHandler is the main handler for the address manager.  It must be run
// as a goroutine.
func (m *Manager) addressHandler() {
	dumpAddressTicker := time.NewTicker(dumpAddressInterval)
	defer dumpAddressTicker.Stop()
out:
	for {
		select {
		case <-dumpAddressTicker.C:
			m.savePeers()

		case <-m.quit:
			break out
		}
	}
	m.savePeers()
	m.wg.Done()
}

func (m *Manager) deserializePeers() error {
	filePath := m.peersFile
	_, err := os.Stat(filePath)
	if os.IsNotExist(err) {
		return nil
	}
	r, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("%s error opening file: %v", filePath, err)
	}
	defer r.Close()

	var nodes map[string]*Node
	dec := json.NewDecoder(r)
	err = dec.Decode(&nodes)
	if err != nil {
		return fmt.Errorf("error reading %s: %v", filePath, err)
	}

	m.mtx.Lock()
	m.nodes = nodes
	m.mtx.Unlock()

	return nil
}

func (m *Manager) savePeers() {
	m.mtx.RLock()
	defer m.mtx.RUnlock()

	// Write temporary peers file and then move it into place.
	tmpfile := m.peersFile + ".new"
	w, err := os.Create(tmpfile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening file %s: %v", tmpfile, err)
		return
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(&m.nodes); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to encode file %s: %v", tmpfile, err)
		return
	}
	if err := w.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "Error closing file %s: %v", tmpfile, err)
		return
	}
	if err := os.Rename(tmpfile, m.peersFile); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file %s: %v", m.peersFile, err)
		return
	}
}

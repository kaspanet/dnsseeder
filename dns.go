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
	"strings"
	"sync/atomic"
	"time"

	"github.com/daglabs/btcd/util/subnetworkid"
	"github.com/daglabs/btcd/wire"
	"github.com/miekg/dns"
)

// DNSServer struct
type DNSServer struct {
	hostname   string
	listen     string
	nameserver string
}

// Start - starts server
func (d *DNSServer) Start() {
	defer wg.Done()

	rr := fmt.Sprintf("%s 86400 IN NS %s", d.hostname, d.nameserver)
	authority, err := dns.NewRR(rr)
	if err != nil {
		log.Printf("NewRR: %v", err)
		return
	}

	udpAddr, err := net.ResolveUDPAddr("udp4", d.listen)
	if err != nil {
		log.Printf("ResolveUDPAddr: %v", err)
		return
	}

	udpListen, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("ListenUDP: %v", err)
		return
	}
	defer udpListen.Close()

	for {
		b := make([]byte, 512)
	mainLoop:
		err := udpListen.SetReadDeadline(time.Now().Add(time.Second))
		if err != nil {
			log.Printf("SetReadDeadline: %v", err)
			os.Exit(1)
		}
		_, addr, err := udpListen.ReadFromUDP(b)
		if err != nil {
			if err, ok := err.(net.Error); ok && err.Timeout() {
				if atomic.LoadInt32(&systemShutdown) == 0 {
					// use goto in order to do not re-allocate 'b' buffer
					goto mainLoop
				}
				log.Printf("DNS server shutdown")
				return
			}
			log.Printf("Read: %T", err.(*net.OpError).Err)
			continue
		}

		wg.Add(1)

		go d.handleDNSRequest(authority, udpListen, b, addr)
	}
}

// NewDNSServer - create DNS server
func NewDNSServer(hostname, nameserver, listen string) *DNSServer {
	if hostname[len(hostname)-1] != '.' {
		hostname = hostname + "."
	}
	if nameserver[len(nameserver)-1] != '.' {
		nameserver = nameserver + "."
	}

	return &DNSServer{
		hostname:   hostname,
		listen:     listen,
		nameserver: nameserver,
	}
}

func (d *DNSServer) handleDNSRequest(authority dns.RR, udpListen *net.UDPConn, b []byte, addr *net.UDPAddr) {
	defer wg.Done()
	dnsMsg := new(dns.Msg)
	err := dnsMsg.Unpack(b[:])
	if err != nil {
		log.Printf("%s: invalid dns message: %v", addr, err)
		return
	}
	if len(dnsMsg.Question) != 1 {
		log.Printf("%s sent more than 1 question: %d", addr, len(dnsMsg.Question))
		return
	}
	domainName := strings.ToLower(dnsMsg.Question[0].Name)
	ff := strings.LastIndex(domainName, d.hostname)
	if ff < 0 {
		log.Printf("invalid name: %s",
			dnsMsg.Question[0].Name)
		return
	}

	// Domain name may be in following format:
	//   [nsubnetwork.][xservice.]hostname
	wantedSF := wire.SFNodeNetwork
	subnetworkID := &wire.SubnetworkIDSupportsAll
	if d.hostname != domainName {
		idx := 0
		labels := dns.SplitDomainName(domainName)
		if labels[0][0] == 'n' && len(labels[0]) > 1 {
			idx = 1
			subnetworkID, err = subnetworkid.NewFromStr(labels[0][1:])
			if err != nil {
				log.Printf("%s: subnetworkid.NewFromStr: %v", addr, err)
				return
			}
		}
		if labels[idx][0] == 'x' && len(labels[idx]) > 1 {
			wantedSFStr := labels[idx][1:]
			u, err := strconv.ParseUint(wantedSFStr, 10, 64)
			if err != nil {
				log.Printf("%s: ParseUint: %v", addr, err)
				return
			}
			wantedSF = wire.ServiceFlag(u)
		}
	}

	var atype string
	qtype := dnsMsg.Question[0].Qtype
	switch qtype {
	case dns.TypeA:
		atype = "A"
	case dns.TypeAAAA:
		atype = "AAAA"
	case dns.TypeNS:
		atype = "NS"
	default:
		log.Printf("%s: invalid qtype: %d", addr,
			dnsMsg.Question[0].Qtype)
		return
	}

	log.Printf("%s: query %d for %v", addr, dnsMsg.Question[0].Qtype, wantedSF)

	respMsg := dnsMsg.Copy()
	respMsg.Authoritative = true
	respMsg.Response = true

	if qtype != dns.TypeNS {
		respMsg.Ns = append(respMsg.Ns, authority)
		addrs := amgr.GoodAddresses(qtype, wantedSF, subnetworkID)
		for _, a := range addrs {
			rr := fmt.Sprintf("%s 30 IN %s %s",
				dnsMsg.Question[0].Name, atype,
				a.IP.String())
			newRR, err := dns.NewRR(rr)
			if err != nil {
				log.Printf("%s: NewRR: %v", addr, err)
				return
			}

			respMsg.Answer = append(respMsg.Answer, newRR)
		}
	} else {
		rr := fmt.Sprintf("%s 86400 IN NS %s", dnsMsg.Question[0].Name, d.nameserver)
		newRR, err := dns.NewRR(rr)
		if err != nil {
			log.Printf("%s: NewRR: %v", addr, err)
			return
		}

		respMsg.Answer = append(respMsg.Answer, newRR)
	}

	//done:
	sendBytes, err := respMsg.Pack()
	if err != nil {
		log.Printf("%s: failed to pack response: %v", addr, err)
		return
	}

	_, err = udpListen.WriteToUDP(sendBytes, addr)
	if err != nil {
		log.Printf("%s: failed to write response: %v", addr, err)
		return
	}
}

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/kaspanet/kaspad/domain/consensus/model/externalapi"
	"github.com/kaspanet/kaspad/infrastructure/config"

	"github.com/kaspanet/kaspad/app/appmessage"
	"github.com/kaspanet/kaspad/infrastructure/network/dnsseed/pb"
	"google.golang.org/grpc"
)

func TestGetPeers(t *testing.T) {
	activeConfig = &ConfigFlags{
		NetworkFlags: config.NetworkFlags{Devnet: true},
	}

	err := activeConfig.NetworkFlags.ResolveNetwork(nil)
	if err != nil {
		t.Fatalf("ResolveNetwork: %s", err)
	}

	peersDefaultPort = 1313

	amgr, err = NewManager(defaultHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	ip := net.IP([]byte{203, 105, 20, 21})
	netAddress := appmessage.NewNetAddressIPPort(ip, uint16(peersDefaultPort))
	amgr.AddAddresses([]*appmessage.NetAddress{netAddress})
	amgr.Good(ip, nil)

	host := "localhost:3737"
	grpcServer := NewGRPCServer(amgr)
	err = grpcServer.Start(host)

	if err != nil {
		t.Fatal("Failed to start gRPC server")
	}

	var subnetworkID *externalapi.DomainSubnetworkID
	conn, err := grpc.Dial(host, grpc.WithInsecure())
	if err != nil {
		t.Logf("Failed to connect to gRPC server: %s", host)
	}

	client := pb.NewPeerServiceClient(conn)
	includeAllSubnetworks := false
	var subnetID []byte
	if subnetworkID != nil {
		subnetID = subnetworkID[:]
	} else {
		subnetID = nil
	}

	req := &pb.GetPeersListRequest{
		SubnetworkID:          subnetID,
		IncludeAllSubnetworks: includeAllSubnetworks,
	}
	res, err := client.GetPeersList(context.Background(), req)

	if err != nil {
		t.Errorf("gRPC request to get peers failed (host=%s): %s", host, err)
		return
	}

	seedPeers := fromProtobufAddresses(res.Addresses)

	numPeers := len(seedPeers)

	t.Logf("%d addresses found from DNS seed %s", numPeers, host)

	if numPeers == 0 {
		t.Error("No peers")
	}

	t.Logf("TestGetPeers completed")
	grpcServer.Stop()
}

func fromProtobufAddresses(proto []*pb.NetAddress) []net.IP {
	var addresses []net.IP

	for _, pbAddr := range proto {
		addresses = append(addresses, pbAddr.IP)
	}

	return addresses
}

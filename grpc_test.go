package main

import (
	"context"
	"fmt"
		pb2 "github.com/kaspanet/kaspad/dnsseed/pb"
"github.com/kaspanet/kaspad/util/subnetworkid"
	"github.com/kaspanet/kaspad/wire"
	"google.golang.org/grpc"
	"net"
	"os"
	"testing"
)

func TestGetPeers(t *testing.T) {
	peersDefaultPort = 1313

	var err error
	amgr, err = NewManager(defaultHomeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "NewManager: %v\n", err)
		os.Exit(1)
	}

	amgr.Good(net.IP([]byte{ 203, 105, 20, 21}), wire.SFNodeNetwork, &subnetworkid.SubnetworkID{})

	grpcServer := NewGRPCServer(amgr)
	err = grpcServer.Start(3737)

	if err != nil {
		t.Fatal("Failed to start gRPC server")
	}

	//defer func() {
	//	log.Infof("Gracefully shutting down the seeder...")
	//	atomic.StoreInt32(&systemShutdown, 1)
	//	close(amgr.quit)
	//	wg.Wait()
	//	amgr.wg.Wait()
	//	log.Infof("Seeder shutdown complete")
	//}()


	host := "localhost:3737"
	var subnetworkID *subnetworkid.SubnetworkID = &subnetworkid.SubnetworkID{}

	conn, err := grpc.Dial(host, grpc.WithInsecure())
	client := pb2.NewPeerServiceClient(conn)
	serviceFlag := wire.SFNodeNetwork
	includeAllSubnetworks := false
	if err != nil {
		t.Logf("Failed to connect to gRPC server: %s", host)
	}

	var subnetID []byte
	if subnetworkID != nil {
		subnetID = subnetworkID.CloneBytes()
	} else {
		subnetID = nil
	}

	req := &pb2.GetPeersListRequest{
		ServiceFlag: uint64(serviceFlag),
		SubnetworkID: subnetID,
		IncludeAllSubnetworks: includeAllSubnetworks,
	}
	t.Error()
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

	t.Logf("finally")
	grpcServer.Stop()
}

func fromProtobufAddresses(proto []*pb2.NetAddress) []net.IP {
	var addresses []net.IP


	for _, pbAddr := range proto {
		addresses = append(addresses, net.IP(pbAddr.IP))
	}

	return addresses
}

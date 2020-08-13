package main

import (
	"fmt"
	"github.com/kaspanet/kaspad/rpc/client"
	"strings"
)

func main() {
	connCfg := &client.ConnConfig{
		Host:         "localhost:16630",
		User:         "test",
		Pass:         "test",
		HTTPPostMode: true,
		DisableTLS:   true,
	}
	cl, err := client.New(connCfg, nil)
	if err != nil {
		panic(err)
	}

	res, err := cl.GetPeerAddresses()

	if err != nil {
		panic(err)
	}

	var ips []string

	for _, address := range res.Addresses {
		ips = append(ips, address.Addr)
	}

	fmt.Printf(strings.Join(ips, ","))
}

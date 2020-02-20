DNSSeeder
====
Warning: This is pre-alpha software. There's no guarantee anything works.
====

[![ISC License](http://img.shields.io/badge/license-ISC-blue.svg)](https://choosealicense.com/licenses/isc/)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](http://godoc.org/github.com/kaspanet/dnsseeder)

DNSSeeder exposes a list of known peers to any new peer joining the Kaspa network via the DNS protocol.

When DNSSeeder is started for the first time, it will connect to the kaspad node
specified with the `-s` flag and listen for `addr` messages. These messages
contain the IPs of all peers known by the node. DNSSeeder will then connect to
each of these peers, listen for their `addr` messages, and continue to traverse
the network in this fashion. DNSSeeder maintains a list of all known peers and
periodically checks that they are online and available. The list is stored on
disk in a json file, so on subsequent start ups the kaspad node specified with
`-s` does not need to be online.

When DNSSeeder is queried for node information, it responds with details of a
random selection of the reliable nodes it knows about.

It is written in Go (golang).

This project is currently under active development and is in a pre-Alpha state. 
Some things still don't work and APIs are far from finalized. The code is provided for reference only.


## Requirements

Latest version of [Go](http://golang.org) (currently 1.13)

## Getting Started

- Install Go according to the installation instructions here:
  http://golang.org/doc/install

- Ensure Go was installed properly and is a supported version:

- Launch a kaspad node for the DNSSeeder to connect to

```bash
$ go version
$ go env GOROOT GOPATH
```

NOTE: The `GOROOT` and `GOPATH` above must not be the same path. It is
recommended that `GOPATH` is set to a directory in your home directory such as
`~/dev/go` to avoid write permission issues. It is also recommended to add
`$GOPATH/bin` to your `PATH` at this point.

- Run the following commands to obtain dnsseeder, all dependencies, and install it:

```bash
$ git clone https://github.com/kaspanet/dnsseeder $GOPATH/src/github.com/kaspanet/dnsseeder
$ cd $GOPATH/src/github.com/kaspanet/dnsseeder
$ go install . 
```

- dnsseeder will now be installed in either ```$GOROOT/bin``` or
  ```$GOPATH/bin``` depending on your configuration. If you did not already
  add the bin directory to your system path during Go installation, we
  recommend you do so now.

To start dnsseeder listening on udp 127.0.0.1:5354 with an initial connection to working testnet node running on 127.0.0.1:

```
$ ./dnsseeder -n nameserver.example.com -H network-seed.example.com -s 127.0.0.1 --testnet
```

You will then need to redirect DNS traffic on your public IP port 53 to 127.0.0.1:5354
Note: to listen directly on port 53 on most Unix systems, one has to run dnsseeder as root, which is discouraged

## Setting up DNS Records

To create a working set-up where the DNSSeeder can provide IPs to kaspad instances, set the following DNS records:
```
NAME                        TYPE        VALUE
----                        ----        -----
[your.domain.name]          A           [your ip address]
[ns-your.domain.name]       NS          [your.domain.name]
```


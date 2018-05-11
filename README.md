dcrseeder
=========

## Requirements

[Go](http://golang.org) 1.9 or newer.

## Getting Started

- dcrseeder will now be installed in either ```$GOROOT/bin``` or
  ```$GOPATH/bin``` depending on your configuration.  If you did not already
  add the bin directory to your system path during Go installation, we
  recommend you do so now.

## Updating

#### Windows

Install a newer MSI

#### Linux/BSD/MacOSX/POSIX - Build from Source

- **Dep**

  Dep is used to manage project dependencies and provide reproducible builds.
  To install:

  `go get -u github.com/golang/dep/cmd/dep`

Unfortunately, the use of `dep` prevents a handy tool such as `go get` from
automatically downloading, building, and installing the source in a single
command.  Instead, the latest project and dependency sources must be first
obtained manually with `git` and `dep`, and then `go` is used to build and
install the project.

**Getting the source**:

For a first time installation, the project and dependency sources can be
obtained manually with `git` and `dep` (create directories as needed):

```
git clone https://github.com/decred/dcrseeder $GOPATH/src/github.com/decred/dcrseeder
cd $GOPATH/src/github.com/decred/dcrseeder
dep ensure
go install . ./cmd/...
```

To update an existing source tree, pull the latest changes and install the
matching dependencies:

```
cd $GOPATH/src/github.com/decred/dcrseeder
git pull
dep ensure
go install . ./cmd/...
```

For more information about Decred and how to set up your software please go to
our docs page at [docs.decred.org](https://docs.decred.org/getting-started/beginner-guide/).

To start dcrseeder listening on udp 127.0.0.1:5354 with an initial connection to working testnet node 192.168.0.1:

```
$ ./dcrseeder -n nameserver.example.com -H network-seed.example.com -s 192.168.0.1 --testnet
```

You will then need to redirect DNS traffic on your public IP port 53 to 127.0.0.1:5354

## Issue Tracker

The [integrated github issue tracker](https://github.com/decred/dcrseeder/issues)
is used for this project.

## License

dcrseeder is licensed under the [copyfree](http://copyfree.org) ISC License.

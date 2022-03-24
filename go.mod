module github.com/kaspanet/dnsseeder

go 1.16

require (
	github.com/jessevdk/go-flags v1.4.0
	github.com/kaspanet/kaspad v0.11.13
	github.com/miekg/dns v1.1.25
	github.com/pkg/errors v0.9.1
	google.golang.org/grpc v1.38.0
)

replace github.com/kaspanet/kaspad => github.com/kaspanet/kaspad v0.11.14

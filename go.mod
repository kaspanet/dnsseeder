module github.com/kaspanet/dnsseeder

go 1.14

require (
	github.com/jessevdk/go-flags v1.4.0
	github.com/kaspanet/kaspad v0.3.1
	github.com/miekg/dns v1.1.25
	github.com/pkg/errors v0.9.1
)

replace github.com/kaspanet/kaspad => ../kaspad

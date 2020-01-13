module github.com/kaspanet/dnsseeder

go 1.13

require (
	github.com/jessevdk/go-flags v1.4.0
	github.com/kaspanet/kaspad v0.1.0
	github.com/miekg/dns v1.1.25
	github.com/pkg/errors v0.8.1
)

replace github.com/kaspanet/kaspad => ../kaspad

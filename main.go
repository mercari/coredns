package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
	_ "github.com/lainra/coredns/plugin/clouddns"
)

var directives = []string{
	"metadata",
	"tls",
	"reload",
	"nsid",
	"root",
	"bind",
	"debug",
	"trace",
	"health",
	"pprof",
	"prometheus",
	"errors",
	"log",
	"dnstap",
	"chaos",
	"loadbalance",
	"cache",
	"rewrite",
	"dnssec",
	"autopath",
	"template",
	"hosts",
	"route53",
	"clouddns",
	"federation",
	"kubernetes",
	"file",
	"auto",
	"secondary",
	"etcd",
	"loop",
	"forward",
	"proxy",
	"erratic",
	"whoami",
	"on",
}

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}

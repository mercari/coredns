package main

import (
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/coremain"
	_ "github.com/lainra/coredns/plugin/clouddns"
)

var directives = []string{
	"example",
	"file",
	"clouddns",
	"whoami",
	"startup",
	"shutdown",
}

func init() {
	dnsserver.Directives = directives
}

func main() {
	coremain.Run()
}

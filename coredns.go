package main

//go:generate go run directives_generate.go

import (
	"github.com/coredns/coredns/coremain"

	// Plug in CoreDNS
	_ "github.com/coredns/coredns/core/plugin"

	_ "github.com/lainra/coredns/plugin/clouddns"
)

func main() {
	coremain.Run()
}

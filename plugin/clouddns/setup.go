package clouddns

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/mholt/caddy"
	gauth "golang.org/x/oauth2/google"

	gdns "google.golang.org/api/dns/v1"
)

var log = clog.NewWithPlugin("clouddns")

func init() {
	caddy.RegisterPlugin("clouddns", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(credential *gauth.Credentials) *CloudDNSClient {
				ctx := context.Background()
				client, err := NewCloudDNSClient(ctx, credential.TokenSource)
				if err != nil {
					fmt.Printf("Failed to create the CloudDNS client: %v", err)
				}
				return client
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(*gauth.Credentials) *CloudDNSClient) error {
	keys := map[string][]string{}
	var credsFilePath string
	var fall fall.F

	up, _ := upstream.New(nil)
	for c.Next() {
		args := c.RemainingArgs()

		for i := range args {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			dns, project := parts[0], parts[1]
			if dns == "" || project == "" {
				return c.Errf("invalid zone '%s'", args[i])
			}

			keys[dns] = append(keys[dns], project)
		}

		for c.NextBlock() {
			switch c.Val() {
			case "upstream":
				args := c.RemainingArgs()
				var err error
				up, err = upstream.New(args)
				if err != nil {
					return c.Errf("invalid upstream: %v", err)
				}
			case "credentials":
				if c.NextArg() {
					credsFilePath = c.Val()
				} else {
					return c.ArgErr()
				}
				if c.NextArg() {
					return c.ArgErr()
				}
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	ctx := context.Background()
	var creds *gauth.Credentials
	if credsFilePath != "" {
		data, err := ioutil.ReadFile(credsFilePath)
		if err != nil {
			return c.Errf("Failed to open the JSON file set in the credentials clause in Corefile: %v", err)
		}
		cred, err := gauth.CredentialsFromJSON(ctx, data, gdns.CloudPlatformScope)
		if err != nil {
			return c.Errf("Unable to get credentials from the specified JSON file: %v", err)
		}
		creds = cred
	} else {
		log.Infof("Not using `credentials` argument, looking for credentials")
		cred, err := gauth.FindDefaultCredentials(ctx, gdns.CloudPlatformScope)
		if err != nil {
			return c.Errf("Unable to acquire auth credentials: %v", err)
		}
		creds = cred
	}

	if creds == nil {
		fmt.Printf("Unable to find any credentials")
	} else {
		if creds.TokenSource == nil {
			log.Warning("Provided credentials don't have a Token Source")
		}
	}
	s := f(creds)

	h, err := New(ctx, s, keys, &up)
	if err != nil {
		return c.Errf("failed to create CloudDNS plugin: %v", err)
	}
	h.Fall = fall
	if err := h.Run(ctx); err != nil {
		return c.Errf("failed to initialize CloudDNS plugin: %v", err)
	}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})

	return nil
}

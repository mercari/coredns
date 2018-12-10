package clouddns

import (
	"context"
	"io/ioutil"
	"strings"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"github.com/mholt/caddy"
	"golang.org/x/oauth2"
	gauth "golang.org/x/oauth2/google"

	gdns "google.golang.org/api/dns/v1"
)

var log = clog.NewWithPlugin("clouddns")

func init() {
	caddy.RegisterPlugin("clouddns", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(credential *gauth.Credentials) {
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(*gauth.Credentials)) error {
	keys := map[string][]string{}
	var credsFilePath string
	// Route53 plugin attempts to find AWS credentials by using ChainCredentials.
	// And the order of that provider chain is as follows:
	// Static AWS keys -> Environment Variables -> Credentials file -> IAM role
	// With that said, even though a user doesn't define any credentials in
	// Corefile, we should still attempt to read the default credentials file,
	// ~/.aws/credentials with the default profile.

	// sharedProvider := &credentials.SharedCredentialsProvider{}
	// var providers []credentials.Provider
	var fall fall.F

	up, _ := upstream.New(nil)
	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
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
			// case "aws_access_key":
			// 	v := c.RemainingArgs()
			// 	if len(v) < 2 {
			// 		return c.Errf("invalid access key '%v'", v)
			// 	}
			// 	providers = append(providers, &credentials.StaticProvider{
			// 		Value: credentials.Value{
			// 			AccessKeyID:     v[0],
			// 			SecretAccessKey: v[1],
			// 		},
			// 	})
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
			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())
			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	ctx := context.Background()
	var creds gauth.Credentials
	if credsFilePath != "" {
		data, err := ioutil.ReadFile(credsFilePath)
		if err != nil {
			log.Fatalf("Failed to open the JSON file: %v", err)
		}
		cred, err := gauth.CredentialsFromJSON(ctx, data, gdns.CloudPlatformScope)
		if err != nil {
			log.Fatalf("Unable to get credentials from the specified JSON file: %v", err)
		}
		creds = *cred
	} else {
		log.Infof("Not using `credentials` argument, looking for credentials")
		cred, err := gauth.FindDefaultCredentials(ctx, gdns.CloudPlatformScope)
		if err != nil {
			log.Fatalf("Unable to acquire auth credentials: %v", err)
		}
		creds = *cred
		log.Info(creds.ProjectID)
		log.Info(creds.TokenSource)
	}
	if creds.ProjectID == "" {
		log.Warning("Provided credentials don't have a GCP Project ID")
		log.Warning(creds.ProjectID)
	}
	project := creds.ProjectID

	if creds.TokenSource == nil {
		log.Warning("Provided credentials don't have a Token Source")
		log.Warning(creds.TokenSource)
	}
	ts := creds.TokenSource

	client := oauth2.NewClient(ctx, ts)

	dnsClient, err := gdns.New(client)
	s := AdaptService(dnsClient)

	h, err := New(ctx, s, project, keys, &up)
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

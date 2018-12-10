# clouddns

## Name

*clouddns* - enables serving zone data from Google Cloud DNS

## Description

The clouddns plugin is useful for serving zones from resource record sets in Google Cloud DNS. This plugin
supports all Google Cloud DNS records specified at <https://cloud.google.com/dns/docs/overview#supported_dns_record_types.>
There is no restriction on where you deploy the clouddns plugin, you only need valid credentials to your GCP account.
Please also understand that this plugin is made to work with one GCP account at a time. (Following the GCP way)
CloudDNS API doesn't allow us to fetch an hosted zone by the domain FQDN, only by the zone name in CloudDNS or its ID.

## Syntax

~~~ txt
clouddns [GCP_PROJECT:GCP_ZONE_NAME/ID...] {
    upstream [ADDRESS...]
    credentials [FILENAME]
    fallthrough [ZONES...]
}
~~~

* **GCP_PROJECT**: the project name owning the hosted zone that contains the resource record sets to be accessed.

* **GCP_ZONE_NAME/ID**: the name OR id of the GCP hosted zone to be accessed. The hosted zone must be part of the project stated in the same key:value pair.

* `upstream` [**ADDRESS**...] specifies upstream resolver(s) used for resolving services that point
  to external hosts (eg. used to resolve CNAMEs). If no **ADDRESS** is given, CoreDNS will resolve
  against itself. **ADDRESS** can be an IP, an IP:port or a path to a file structured like
  resolv.conf.

* `credentials`: Used to read the credential file and feeding CoreDNS with the proper service account to use to fetch hosted zone data.

* **FILENAME** GCP JSON service account credentials filename. Defaults to the value of `GOOGLE_AUTH_CREDENTIALS` environment variable.

* `fallthrough`: If zone matches and no record can be generated, pass request to the next plugin.
  If **[ZONES...]** is omitted, then fallthrough happens for all zones for which the plugin
  is authoritative. If specific zones are listed (for example `in-addr.arpa` and `ip6.arpa`), then only
  queries for those zones will be subject to fallthrough.

* **ZONES**: Hosted zones it should be authoritative for. If empty, the zones from the configuration block are chosen.

## Examples

Enable clouddns with implicit GCP credentials:

~~~ txt
. {
  clouddns myproject:myhostedzonename
}
~~~

Enable clouddns with implicit GCP credentials and multiple zones.
Typically used to serve both an hosted zone and its reverse zone.
Note that you can use different projects,
as long as your credentials are valid for each of them. Take one key:value per project/hosted zone:

~~~ txt
. {
    clouddns myproject:myfirsthostedzonename myproject:myfirstreversehostedzonename myotherproject:mysecondhostedzonename
}
~~~

Enable clouddns with implicit GCP credentials and an upstream:

~~~ txt
. {
      clouddns myproject:myhostedzonename {
      upstream 10.0.0.1
  }
}
~~~

Enable clouddns with explicit GCP credentials file path:

~~~ txt
. {
    clouddns myproject:myhostedzonename {
      credentials /home/.config/gcp-sa.json
    }
}
~~~

Enable clouddns with fallthrough. Please note that the fallthrough directive takes the hosted zone FQDN as a parameter:

~~~ txt
. {
    clouddns myproject:myfirsthostedzonename myproject:mysecondhostedzonename {
      fallthrough mysecondhostedzonenameFQDN
    }
}
~~~
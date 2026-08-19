// Package templates provides canned site-block skeletons for the "new
// site" wizard, matching the reverse-proxy + Cloudflare-DNS-TLS + optional
// SSO-import patterns already used throughout the target Caddyfile.
package templates

import "fmt"

const indent = "    "

type Template struct {
	ID            string
	Name          string
	Description   string
	NeedsUpstream bool
	Build         func(domain, upstream, comment string) []string
}

func tlsBlock(depth int) []string {
	pad := ""
	for i := 0; i < depth; i++ {
		pad += indent
	}
	return []string{
		pad + "tls {",
		pad + indent + "dns cloudflare {env.CF_API_TOKEN}",
		pad + "}",
	}
}

func header(domain, comment string) []string {
	var lines []string
	if comment != "" {
		lines = append(lines, "# "+comment)
	}
	lines = append(lines, domain+" {")
	return lines
}

var All = []Template{
	{
		ID:            "plain",
		Name:          "Plain reverse_proxy",
		Description:   "reverse_proxy + Cloudflare DNS TLS, no SSO",
		NeedsUpstream: true,
		Build: func(domain, upstream, comment string) []string {
			out := header(domain, comment)
			out = append(out, indent+"reverse_proxy "+upstream)
			out = append(out, tlsBlock(1)...)
			out = append(out, "}")
			return out
		},
	},
	{
		ID:            "sso-core",
		Name:          "SSO core (Authelia)",
		Description:   "import sso-core <upstream> + TLS",
		NeedsUpstream: true,
		Build: func(domain, upstream, comment string) []string {
			out := header(domain, comment)
			out = append(out, indent+"import sso-core "+upstream)
			out = append(out, tlsBlock(1)...)
			out = append(out, "}")
			return out
		},
	},
	{
		ID:            "sso-core-basic",
		Name:          "SSO core + Basic auth upstream",
		Description:   "import sso-core-basic <upstream> + TLS",
		NeedsUpstream: true,
		Build: func(domain, upstream, comment string) []string {
			out := header(domain, comment)
			out = append(out, indent+"import sso-core-basic "+upstream)
			out = append(out, tlsBlock(1)...)
			out = append(out, "}")
			return out
		},
	},
	{
		ID:            "sso-bypass",
		Name:          "SSO with bypass key (e.g. mobile client)",
		Description:   "import sso-bypass <upstream> + TLS",
		NeedsUpstream: true,
		Build: func(domain, upstream, comment string) []string {
			out := header(domain, comment)
			out = append(out, indent+"import sso-bypass "+upstream)
			out = append(out, tlsBlock(1)...)
			out = append(out, "}")
			return out
		},
	},
	{
		ID:            "sso-bypass-basic",
		Name:          "SSO bypass + Basic auth upstream",
		Description:   "import sso-bypass-basic <upstream> + TLS",
		NeedsUpstream: true,
		Build: func(domain, upstream, comment string) []string {
			out := header(domain, comment)
			out = append(out, indent+"import sso-bypass-basic "+upstream)
			out = append(out, tlsBlock(1)...)
			out = append(out, "}")
			return out
		},
	},
	{
		ID:            "custom",
		Name:          "Empty skeleton",
		Description:   "Just a domain block + TLS, fill in the rest by hand",
		NeedsUpstream: false,
		Build: func(domain, upstream, comment string) []string {
			out := header(domain, comment)
			out = append(out, indent)
			out = append(out, tlsBlock(1)...)
			out = append(out, "}")
			return out
		},
	},
}

func ByID(id string) (Template, error) {
	for _, t := range All {
		if t.ID == id {
			return t, nil
		}
	}
	return Template{}, fmt.Errorf("unknown template %q", id)
}

// Package profile renders the two Apple configuration profiles for the
// family devices (#41): one for the kids' supervised iPads and one for the
// adults' iPhones. Both point the device's DNS at a Cloudflare Gateway
// location over HTTPS; the kids' profile also locks the iPad down and the
// adults' profile carries the Gateway root certificate so Gateway can
// inspect HTTPS through the Cloudflare One Agent.
//
// The profiles are plain XML property lists built with a plist library, so
// they are well formed by construction, and every identifier and UUID is
// fixed: two renders with the same inputs are byte for byte the same, and
// installing a new render replaces the installed profile instead of adding
// a second one. The /admin page serves what this package renders; the
// `site profile` command writes it to disk.
//
// Two notes from Apple's documentation that shape the design: a DNS
// Settings profile installed by hand (Safari or Apple Configurator) applies
// on cellular as well as Wi-Fi, where one pushed by an MDM applies only to
// managed Wi-Fi; and Apple marks com.apple.dnsSettings.managed and
// allowListedAppBundleIDs deprecated as of iOS 27 in favour of declarative
// device management, so they work through iOS 26 and the replacement is
// known.
package profile

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/micromdm/plist"
)

// The file names the /admin page serves and `site profile` writes.
const (
	KidsFileName  = "kids-ipad.mobileconfig"
	AdultFileName = "adult-iphone.mobileconfig"
)

// Every payload identifier starts here; iOS decides from the top-level
// identifier whether a new install replaces the installed profile.
const identifierPrefix = "com.craigdevjohnson.family."

// Shown on the install screen as the profile's source.
const organization = "The Lobby (craigdevjohnson.com)"

// The name the adult profile gives the root certificate.
const certificateFileName = "cloudflare-gateway-ca.cer"

// Every PayloadUUID is a version 5 UUID of the payload's identifier in this
// namespace, so no state file is needed to keep them stable.
var namespace = uuid.NewSHA1(uuid.NameSpaceURL, []byte("https://craigdevjohnson.com/admin/profiles"))

// A Gateway DNS location's DNS over HTTPS endpoint, and nothing else.
var dohURLPattern = regexp.MustCompile(`^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?\.cloudflare-gateway\.com/dns-query$`)

// ErrNotConfigured says an input the profile needs is unset, which is the
// state before the Zero Trust configuration (#43) has been applied.
var ErrNotConfigured = errors.New("profile: not configured")

// ParseDoHURL accepts a Cloudflare Gateway location's DNS over HTTPS
// endpoint, https://<subdomain>.cloudflare-gateway.com/dns-query, and
// refuses anything else: a profile pointing anywhere else would send the
// household's DNS to a stranger.
func ParseDoHURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("%w: no DNS over HTTPS address", ErrNotConfigured)
	}
	if !dohURLPattern.MatchString(s) {
		return "", fmt.Errorf("profile: %q is not a Gateway location's DNS over HTTPS address (https://<subdomain>.cloudflare-gateway.com/dns-query)", s)
	}
	return s, nil
}

// ParseCertificate accepts exactly one PEM-encoded X.509 certificate, the
// account's Gateway root certificate, and returns its DER bytes for the
// certificate payload. pem.Decode reports "no PEM" with a nil block rather
// than an error, so that case is checked by hand.
func ParseCertificate(pemText string) ([]byte, error) {
	if strings.TrimSpace(pemText) == "" {
		return nil, fmt.Errorf("%w: no Gateway root certificate", ErrNotConfigured)
	}
	block, rest := pem.Decode([]byte(pemText))
	if block == nil {
		return nil, errors.New("profile: the Gateway root certificate is not PEM")
	}
	if block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("profile: the Gateway root certificate is a %q block, want CERTIFICATE", block.Type)
	}
	if strings.TrimSpace(string(rest)) != "" {
		return nil, errors.New("profile: the Gateway root certificate must be exactly one certificate")
	}
	if _, err := x509.ParseCertificate(block.Bytes); err != nil {
		return nil, fmt.Errorf("profile: the Gateway root certificate does not parse: %w", err)
	}
	return block.Bytes, nil
}

// Kids is the profile for a supervised iPad: DNS over HTTPS to the Kids
// Gateway location that the child cannot switch off, the restrictions that
// keep the filter in place, and Apple's built-in web content filter as a
// second layer in Safari. Installed through Apple Configurator so that it
// lands with removal disallowed.
type Kids struct {
	// DoHURL is the Kids location's endpoint; see ParseDoHURL.
	DoHURL string
}

// Adult is the profile for an unsupervised iPhone: the Gateway root
// certificate, so Gateway can inspect HTTPS through the Cloudflare One
// Agent, and DNS over HTTPS to the Adults location for when the agent is
// off. It locks nothing: the phones' real filter is the agent.
type Adult struct {
	// DoHURL is the Adults location's endpoint; see ParseDoHURL.
	DoHURL string
	// GatewayRootCA is the account's Gateway root certificate as PEM; see
	// ParseCertificate.
	GatewayRootCA string
}

// Render builds the kids' profile. It fails with ErrNotConfigured while the
// endpoint is unset, so nothing with a placeholder in it is ever written.
func (k Kids) Render() ([]byte, error) {
	doh, err := ParseDoHURL(k.DoHURL)
	if err != nil {
		return nil, err
	}
	const id = identifierPrefix + "kids-ipad"
	return render(configuration{
		header: newHeader("Configuration", id, "Family iPad",
			"Filters this iPad's internet through the family's Cloudflare Gateway, keeps that filter from being switched off, and asks a parent before a new app is installed."),
		// Honoured for a profile installed by hand only on a supervised
		// device, which the iPads are.
		PayloadRemovalDisallowed: true,
		TargetDeviceType:         targetIPhoneAndIPad,
		PayloadContent: []any{
			dnsPayload{
				header: newHeader("com.apple.dnsSettings.managed", id+".dns", "Family DNS (kids)",
					"Sends this iPad's DNS lookups to the family's Cloudflare Gateway over HTTPS."),
				DNSSettings: dnsSettings{DNSProtocol: "HTTPS", ServerURL: doh},
				// Supervised only. AllowFailover stays unset so the iPad
				// fails closed if Gateway is unreachable, which is the
				// point of the lock.
				ProhibitDisablement: true,
			},
			restrictionsPayload{
				header: newHeader("com.apple.applicationaccess", id+".restrictions", "Family restrictions",
					"Stops VPN apps, removing apps, erasing the iPad, installing profiles and changing accounts. Installing apps stays on, gated by Ask to Buy."),
				AllowVPNCreation:                        false,
				AllowAppRemoval:                         false,
				AllowEraseContentAndSettings:            false,
				AllowUIConfigurationProfileInstallation: false,
				AllowAccountModification:                false,
				AllowAppInstallation:                    true,
				// allowListedAppBundleIDs is deliberately absent: the app
				// allow-list waits for an MDM (#40). When one arrives, the
				// key goes here as an array of bundle ids.
			},
			webFilterPayload{
				header: newHeader("com.apple.webcontent-filter", id+".webfilter", "Family web filter",
					"Turns on Apple's built-in filter for adult websites in Safari, under the family's DNS filter."),
				FilterType:        "BuiltIn",
				AutoFilterEnabled: true,
				// Empty to start (#40); additions come from the Gateway
				// activity log. Matching is by substring, ignoring a
				// leading www.
				PermittedURLs: []string{},
				DenyListURLs:  []string{},
			},
		},
	})
}

// Render builds the adults' profile. It fails with ErrNotConfigured while
// the endpoint or the certificate is unset.
func (a Adult) Render() ([]byte, error) {
	doh, err := ParseDoHURL(a.DoHURL)
	if err != nil {
		return nil, err
	}
	der, err := ParseCertificate(a.GatewayRootCA)
	if err != nil {
		return nil, err
	}
	const id = identifierPrefix + "adult-iphone"
	return render(configuration{
		header: newHeader("Configuration", id, "Family iPhone",
			"Trusts the family's Cloudflare Gateway certificate so the Cloudflare One Agent can check web traffic, and uses the family's DNS when the agent is off. After installing, turn on full trust for the certificate in Settings > General > About > Certificate Trust Settings."),
		TargetDeviceType: targetIPhoneAndIPad,
		PayloadContent: []any{
			certificatePayload{
				header: newHeader("com.apple.security.root", id+".ca", "Cloudflare Gateway certificate",
					"The family's Cloudflare Gateway root certificate. Full trust is turned on by hand in Settings after installing."),
				PayloadContent:             der,
				PayloadCertificateFileName: certificateFileName,
			},
			dnsPayload{
				header: newHeader("com.apple.dnsSettings.managed", id+".dns", "Family DNS (adults)",
					"Sends this iPhone's DNS lookups to the family's Cloudflare Gateway over HTTPS while the Cloudflare One Agent is off."),
				// AllowFailover (iOS 26 and later) lets the phone fall back
				// to the carrier's DNS if Gateway is unreachable while the
				// agent is off (#40). Unsupervised, so no ProhibitDisablement.
				DNSSettings: dnsSettings{DNSProtocol: "HTTPS", ServerURL: doh, AllowFailover: true},
			},
		},
	})
}

func render(c configuration) ([]byte, error) {
	out, err := plist.MarshalIndent(c, "\t")
	if err != nil {
		return nil, fmt.Errorf("profile: %w", err)
	}
	return out, nil
}

// targetIPhoneAndIPad is TargetDeviceType for iOS and iPadOS, so an iPhone
// with a paired Apple Watch is not asked which device the profile is for.
const targetIPhoneAndIPad = 1

// header holds the keys every payload and the profile itself carry. The
// UUID is derived from the identifier, so it is as fixed as the identifier.
type header struct {
	PayloadType         string `plist:"PayloadType"`
	PayloadVersion      int    `plist:"PayloadVersion"`
	PayloadIdentifier   string `plist:"PayloadIdentifier"`
	PayloadUUID         string `plist:"PayloadUUID"`
	PayloadDisplayName  string `plist:"PayloadDisplayName"`
	PayloadDescription  string `plist:"PayloadDescription"`
	PayloadOrganization string `plist:"PayloadOrganization"`
}

func newHeader(payloadType, identifier, name, description string) header {
	return header{
		PayloadType:         payloadType,
		PayloadVersion:      1,
		PayloadIdentifier:   identifier,
		PayloadUUID:         strings.ToUpper(uuid.NewSHA1(namespace, []byte(identifier)).String()),
		PayloadDisplayName:  name,
		PayloadDescription:  description,
		PayloadOrganization: organization,
	}
}

// configuration is the top level of a profile.
type configuration struct {
	header
	PayloadContent           []any `plist:"PayloadContent"`
	PayloadRemovalDisallowed bool  `plist:"PayloadRemovalDisallowed,omitempty"`
	TargetDeviceType         int   `plist:"TargetDeviceType"`
}

// dnsPayload is com.apple.dnsSettings.managed. In Apple's schema
// ProhibitDisablement is a sibling of the DNSSettings dictionary while
// AllowFailover sits inside it; each is written only when true, so the
// kids' payload has no AllowFailover and the adults' no ProhibitDisablement.
type dnsPayload struct {
	header
	DNSSettings         dnsSettings `plist:"DNSSettings"`
	ProhibitDisablement bool        `plist:"ProhibitDisablement,omitempty"`
}

type dnsSettings struct {
	DNSProtocol   string `plist:"DNSProtocol"`
	ServerURL     string `plist:"ServerURL"`
	AllowFailover bool   `plist:"AllowFailover,omitempty"`
}

// restrictionsPayload is com.apple.applicationaccess, with exactly the keys
// the kids' iPads need, spelled as Apple spells them (checked against
// Apple's device-management schema; the handoff's
// allowConfigurationProfileInstallation is not a key).
type restrictionsPayload struct {
	header
	AllowVPNCreation                        bool `plist:"allowVPNCreation"`
	AllowAppRemoval                         bool `plist:"allowAppRemoval"`
	AllowEraseContentAndSettings            bool `plist:"allowEraseContentAndSettings"`
	AllowUIConfigurationProfileInstallation bool `plist:"allowUIConfigurationProfileInstallation"`
	AllowAccountModification                bool `plist:"allowAccountModification"`
	AllowAppInstallation                    bool `plist:"allowAppInstallation"`
}

// webFilterPayload is com.apple.webcontent-filter. DenyListURLs is the
// current name of the list Apple called BlacklistedURLs before iOS 14.5;
// PermittedURLs needs AutoFilterEnabled, which is on.
type webFilterPayload struct {
	header
	FilterType        string   `plist:"FilterType"`
	AutoFilterEnabled bool     `plist:"AutoFilterEnabled"`
	PermittedURLs     []string `plist:"PermittedURLs"`
	DenyListURLs      []string `plist:"DenyListURLs"`
}

// certificatePayload is com.apple.security.root: a root certificate the
// device is asked to trust. iOS installs it but leaves full trust off until
// it is turned on by hand in Settings.
type certificatePayload struct {
	header
	PayloadContent             []byte `plist:"PayloadContent"`
	PayloadCertificateFileName string `plist:"PayloadCertificateFileName"`
}

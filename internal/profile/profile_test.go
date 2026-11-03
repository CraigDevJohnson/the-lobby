package profile

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/micromdm/plist"
)

const (
	kidsDoH   = "https://kids0123456789.cloudflare-gateway.com/dns-query"
	adultsDoH = "https://adults0123456789.cloudflare-gateway.com/dns-query"
)

// testCertificate makes a self-signed CA like the one Cloudflare issues per
// account, as PEM and as the DER the payload must carry.
func testCertificate(t *testing.T) (pemText string, der []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Gateway CA (test)"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err = x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), der
}

// parse reads a rendered profile back as a dictionary, which is also the
// well-formedness check CI can make without plutil.
func parse(t *testing.T, out []byte) map[string]any {
	t.Helper()
	if !bytes.HasPrefix(out, []byte(`<?xml version="1.0" encoding="UTF-8"?>`)) {
		t.Fatalf("not an XML plist: %.80q", out)
	}
	if !bytes.Contains(out, []byte(`<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"`)) {
		t.Fatalf("no plist DOCTYPE: %.200q", out)
	}
	var top map[string]any
	if err := plist.Unmarshal(out, &top); err != nil {
		t.Fatalf("does not parse back: %v", err)
	}
	return top
}

// payloads returns the profile's payloads keyed by PayloadType.
func payloads(t *testing.T, top map[string]any) map[string]map[string]any {
	t.Helper()
	content, ok := top["PayloadContent"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("PayloadContent = %#v, want a non-empty array", top["PayloadContent"])
	}
	byType := map[string]map[string]any{}
	for _, p := range content {
		d, ok := p.(map[string]any)
		if !ok {
			t.Fatalf("payload %#v is not a dictionary", p)
		}
		typ, _ := d["PayloadType"].(string)
		if _, dup := byType[typ]; dup || typ == "" {
			t.Fatalf("payload type %q missing or repeated", typ)
		}
		byType[typ] = d
	}
	return byType
}

func TestKidsProfile(t *testing.T) {
	out, err := Kids{DoHURL: kidsDoH}.Render()
	if err != nil {
		t.Fatal(err)
	}
	top := parse(t, out)
	if top["PayloadType"] != "Configuration" {
		t.Fatalf("PayloadType = %v", top["PayloadType"])
	}
	if top["PayloadRemovalDisallowed"] != true {
		t.Fatalf("PayloadRemovalDisallowed = %v, want true on the kids profile", top["PayloadRemovalDisallowed"])
	}
	if top["PayloadIdentifier"] != "com.craigdevjohnson.family.kids-ipad" || top["TargetDeviceType"] != uint64(1) {
		t.Fatalf("identifier %v, TargetDeviceType %v", top["PayloadIdentifier"], top["TargetDeviceType"])
	}
	commonKeys(t, "profile", top)

	ps := payloads(t, top)
	if len(ps) != 3 {
		t.Fatalf("payload types %v, want DNS, restrictions and web filter", keysOf(ps))
	}
	for typ, p := range ps {
		commonKeys(t, typ, p)
		if id, _ := p["PayloadIdentifier"].(string); !strings.HasPrefix(id, "com.craigdevjohnson.family.kids-ipad.") {
			t.Errorf("%s: identifier %q is not under the profile's", typ, id)
		}
	}

	dns := ps["com.apple.dnsSettings.managed"]
	settings, _ := dns["DNSSettings"].(map[string]any)
	if settings["DNSProtocol"] != "HTTPS" || settings["ServerURL"] != kidsDoH {
		t.Fatalf("DNSSettings = %#v", dns["DNSSettings"])
	}
	if dns["ProhibitDisablement"] != true {
		t.Fatalf("ProhibitDisablement = %v, want true as a sibling of DNSSettings", dns["ProhibitDisablement"])
	}
	if _, has := settings["ProhibitDisablement"]; has {
		t.Fatal("ProhibitDisablement must not be inside DNSSettings")
	}
	if _, has := settings["AllowFailover"]; has {
		t.Fatal("the kids DNS payload must have no AllowFailover: the iPad fails closed")
	}
	if _, has := dns["AllowFailover"]; has {
		t.Fatal("AllowFailover is a DNSSettings key, never a sibling of it")
	}

	restrictions := ps["com.apple.applicationaccess"]
	want := map[string]any{
		"allowVPNCreation":                        false,
		"allowAppRemoval":                         false,
		"allowEraseContentAndSettings":            false,
		"allowUIConfigurationProfileInstallation": false,
		"allowAccountModification":                false,
		"allowAppInstallation":                    true,
	}
	got := map[string]any{}
	for k, v := range restrictions {
		if strings.HasPrefix(k, "allow") {
			got[k] = v
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("restrictions = %#v, want exactly %#v", got, want)
	}
	if _, has := restrictions["allowListedAppBundleIDs"]; has {
		t.Fatal("allowListedAppBundleIDs is deferred with the MDM and must be absent")
	}

	filter := ps["com.apple.webcontent-filter"]
	if filter["FilterType"] != "BuiltIn" || filter["AutoFilterEnabled"] != true {
		t.Fatalf("web filter = %#v", filter)
	}
	for _, list := range []string{"PermittedURLs", "DenyListURLs"} {
		if v, ok := filter[list].([]any); !ok || len(v) != 0 {
			t.Fatalf("%s = %#v, want an empty array", list, filter[list])
		}
	}
	if _, has := filter["BlacklistedURLs"]; has {
		t.Fatal("BlacklistedURLs is the deprecated name; DenyListURLs is the key")
	}
}

func TestAdultProfile(t *testing.T) {
	caPEM, der := testCertificate(t)
	out, err := Adult{DoHURL: adultsDoH, GatewayRootCA: caPEM}.Render()
	if err != nil {
		t.Fatal(err)
	}
	top := parse(t, out)
	if top["PayloadType"] != "Configuration" {
		t.Fatalf("PayloadType = %v", top["PayloadType"])
	}
	if _, has := top["PayloadRemovalDisallowed"]; has {
		t.Fatal("the adult profile must not disallow its own removal")
	}
	if top["PayloadIdentifier"] != "com.craigdevjohnson.family.adult-iphone" || top["TargetDeviceType"] != uint64(1) {
		t.Fatalf("identifier %v, TargetDeviceType %v", top["PayloadIdentifier"], top["TargetDeviceType"])
	}
	commonKeys(t, "profile", top)

	ps := payloads(t, top)
	if len(ps) != 2 {
		t.Fatalf("payload types %v, want the certificate and DNS", keysOf(ps))
	}
	for typ, p := range ps {
		commonKeys(t, typ, p)
	}

	cert := ps["com.apple.security.root"]
	if got, _ := cert["PayloadContent"].([]byte); !bytes.Equal(got, der) {
		t.Fatalf("certificate PayloadContent is not the input certificate's DER (%d bytes vs %d)", len(got), len(der))
	}
	if cert["PayloadCertificateFileName"] != "cloudflare-gateway-ca.cer" {
		t.Fatalf("PayloadCertificateFileName = %v", cert["PayloadCertificateFileName"])
	}

	dns := ps["com.apple.dnsSettings.managed"]
	settings, _ := dns["DNSSettings"].(map[string]any)
	if settings["DNSProtocol"] != "HTTPS" || settings["ServerURL"] != adultsDoH {
		t.Fatalf("DNSSettings = %#v", dns["DNSSettings"])
	}
	if settings["AllowFailover"] != true {
		t.Fatalf("DNSSettings.AllowFailover = %v, want true on the adult profile", settings["AllowFailover"])
	}
	if _, has := dns["AllowFailover"]; has {
		t.Fatal("AllowFailover is a DNSSettings key, never a sibling of it")
	}
	if _, has := dns["ProhibitDisablement"]; has {
		t.Fatal("the adult DNS payload must not prohibit disablement: the phones are unsupervised")
	}
}

// The certificate payload belongs to the adult profile only.
func TestOnlyTheAdultProfileCarriesTheCertificate(t *testing.T) {
	out, err := Kids{DoHURL: kidsDoH}.Render()
	if err != nil {
		t.Fatal(err)
	}
	if _, has := payloads(t, parse(t, out))["com.apple.security.root"]; has {
		t.Fatal("the kids profile must carry no certificate")
	}
}

// Two renders with the same inputs are byte for byte the same, so a new
// install replaces the installed profile rather than adding a second one,
// and the two profiles never share an identifier or a UUID.
func TestRendersAreStable(t *testing.T) {
	caPEM, _ := testCertificate(t)
	kids1, _ := Kids{DoHURL: kidsDoH}.Render()
	kids2, _ := Kids{DoHURL: kidsDoH}.Render()
	if !bytes.Equal(kids1, kids2) {
		t.Fatal("two kids renders differ")
	}
	adult1, _ := Adult{DoHURL: adultsDoH, GatewayRootCA: caPEM}.Render()
	adult2, _ := Adult{DoHURL: adultsDoH, GatewayRootCA: caPEM}.Render()
	if !bytes.Equal(adult1, adult2) {
		t.Fatal("two adult renders differ")
	}

	seen := map[string]string{}
	for name, out := range map[string][]byte{"kids": kids1, "adult": adult1} {
		top := parse(t, out)
		all := []map[string]any{top}
		for _, p := range payloads(t, top) {
			all = append(all, p)
		}
		for _, d := range all {
			for _, key := range []string{"PayloadIdentifier", "PayloadUUID"} {
				v, _ := d[key].(string)
				if where, dup := seen[v]; dup {
					t.Errorf("%s %q appears in %s and %s", key, v, where, name)
				}
				seen[v] = name
			}
			if u, _ := d["PayloadUUID"].(string); len(u) != 36 || u != strings.ToUpper(u) {
				t.Errorf("PayloadUUID %q is not an upper-case UUID", u)
			}
		}
	}
}

func TestInputsAreChecked(t *testing.T) {
	caPEM, _ := testCertificate(t)
	for name, url := range map[string]string{
		"not cloudflare":  "https://dns.google/dns-query",
		"http":            "http://kids.cloudflare-gateway.com/dns-query",
		"wrong path":      "https://kids.cloudflare-gateway.com/",
		"extra host part": "https://kids.cloudflare-gateway.com.evil.example/dns-query",
		"query":           "https://kids.cloudflare-gateway.com/dns-query?x=1",
		"upper case":      "https://KIDS.cloudflare-gateway.com/dns-query",
	} {
		if _, err := (Kids{DoHURL: url}).Render(); err == nil || errors.Is(err, ErrNotConfigured) {
			t.Errorf("%s: %q accepted (err %v)", name, url, err)
		}
		if _, err := (Adult{DoHURL: url, GatewayRootCA: caPEM}).Render(); err == nil || errors.Is(err, ErrNotConfigured) {
			t.Errorf("%s: %q accepted by the adult profile (err %v)", name, url, err)
		}
	}
	if _, err := ParseDoHURL("  " + kidsDoH + "\n"); err != nil {
		t.Errorf("surrounding whitespace refused: %v", err)
	}

	if _, err := (Kids{}).Render(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("empty kids endpoint: err %v, want ErrNotConfigured", err)
	}
	if _, err := (Adult{GatewayRootCA: caPEM}).Render(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("empty adults endpoint: err %v, want ErrNotConfigured", err)
	}
	if _, err := (Adult{DoHURL: adultsDoH}).Render(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("missing certificate: err %v, want ErrNotConfigured", err)
	}

	for name, cert := range map[string]string{
		"not pem":       "-----BEGIN NOTHING",
		"wrong block":   string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: []byte{1, 2, 3}})),
		"garbage der":   string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{1, 2, 3}})),
		"two certs":     caPEM + caPEM,
		"trailing text": caPEM + "and more",
	} {
		if _, err := (Adult{DoHURL: adultsDoH, GatewayRootCA: cert}).Render(); err == nil || errors.Is(err, ErrNotConfigured) {
			t.Errorf("%s: accepted (err %v)", name, err)
		}
	}
	if _, err := ParseCertificate("\n" + caPEM + "\n\n"); err != nil {
		t.Errorf("surrounding whitespace refused: %v", err)
	}
}

// commonKeys checks the keys every payload and the profile carry.
func commonKeys(t *testing.T, what string, d map[string]any) {
	t.Helper()
	for _, key := range []string{"PayloadType", "PayloadIdentifier", "PayloadUUID", "PayloadDisplayName", "PayloadDescription", "PayloadOrganization"} {
		if v, _ := d[key].(string); v == "" {
			t.Errorf("%s: %s missing", what, key)
		}
	}
	if d["PayloadVersion"] != uint64(1) {
		t.Errorf("%s: PayloadVersion = %#v, want 1", what, d["PayloadVersion"])
	}
}

func keysOf(m map[string]map[string]any) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

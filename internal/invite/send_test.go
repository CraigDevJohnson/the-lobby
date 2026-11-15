package invite

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"errors"
	"io"
	"math/big"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"sync"
	"testing"
	"time"
)

const from = "The Lobby <lobby@craigdevjohnson.com>"

// fakeSMTP is an SMTP server on 127.0.0.1 with TLS from the first byte, as
// Fastmail's port 465 is. It takes one exchange, records each command and
// the message, and answers RCPT with rcpt and QUIT with quit.
type fakeSMTP struct {
	addr   string
	client *tls.Config
	rcpt   string
	quit   string

	mu       sync.Mutex
	commands []string
	data     []byte
	done     chan struct{}
}

func newFakeSMTP(t *testing.T, rcpt, quit string) *fakeSMTP {
	t.Helper()
	server, client := testTLS(t)
	ln, err := tls.Listen("tcp", "127.0.0.1:0", server)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	f := &fakeSMTP{addr: ln.Addr().String(), client: client, rcpt: rcpt, quit: quit, done: make(chan struct{})}
	go f.serve(ln)
	return f
}

func (f *fakeSMTP) serve(ln net.Listener) {
	defer close(f.done)
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	c := textproto.NewConn(conn)
	_ = c.PrintfLine("220 localhost ESMTP")
	for {
		line, err := c.ReadLine()
		if err != nil {
			return
		}
		f.mu.Lock()
		f.commands = append(f.commands, line)
		f.mu.Unlock()
		verb, _, _ := strings.Cut(line, " ")
		switch strings.ToUpper(verb) {
		case "EHLO":
			_ = c.PrintfLine("250-localhost")
			_ = c.PrintfLine("250 AUTH PLAIN")
		case "AUTH":
			_ = c.PrintfLine("235 2.7.0 Authentication successful")
		case "MAIL":
			_ = c.PrintfLine("250 2.1.0 Ok")
		case "RCPT":
			_ = c.PrintfLine("%s", f.rcpt)
		case "DATA":
			_ = c.PrintfLine("354 End data with <CR><LF>.<CR><LF>")
			data, err := c.ReadDotBytes()
			if err != nil {
				return
			}
			f.mu.Lock()
			f.data = data
			f.mu.Unlock()
			_ = c.PrintfLine("250 2.0.0 Ok: queued")
		case "QUIT":
			_ = c.PrintfLine("%s", f.quit)
			return
		default:
			_ = c.PrintfLine("502 5.5.2 Unknown command")
		}
	}
}

func (f *fakeSMTP) sender() *Sender {
	return &Sender{
		From:  from,
		Login: func(context.Context) (string, string, error) { return "craig@example.net", "app-password", nil },
		Addr:  f.addr,
		TLS:   f.client,
	}
}

// testTLS makes a certificate for 127.0.0.1 and the client settings that
// trust it.
func testTLS(t *testing.T) (server, client *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "localhost"},
		IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cert)
	return &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}}, &tls.Config{RootCAs: pool}
}

// An invite goes in one exchange over TLS: PLAIN auth with the stored login,
// MAIL FROM The Lobby's bare address, RCPT TO the Member, the message with
// its headers and a quoted-printable UTF-8 body, then QUIT.
func TestSendDeliversTheInvite(t *testing.T) {
	f := newFakeSMTP(t, "250 2.1.5 Ok", "221 2.0.0 Bye")
	e := New(base, "friend@example.com", []string{"minecraft"}, nil)
	if err := f.sender().Send(context.Background(), e); err != nil {
		t.Fatal(err)
	}
	<-f.done

	auth := base64.StdEncoding.EncodeToString([]byte("\x00craig@example.net\x00app-password"))
	want := []string{"EHLO craigdevjohnson.com", "AUTH PLAIN " + auth, "MAIL FROM:<lobby@craigdevjohnson.com>", "RCPT TO:<friend@example.com>", "DATA", "QUIT"}
	if strings.Join(f.commands, "\n") != strings.Join(want, "\n") {
		t.Errorf("commands:\n%s\nwant:\n%s", strings.Join(f.commands, "\n"), strings.Join(want, "\n"))
	}

	msg, err := mail.ReadMessage(bytes.NewReader(f.data))
	if err != nil {
		t.Fatalf("%v in:\n%s", err, f.data)
	}
	for name, value := range map[string]string{
		"From":                      from,
		"To":                        "friend@example.com",
		"Subject":                   "You're invited to The Lobby",
		"Mime-Version":              "1.0",
		"Content-Type":              "text/plain; charset=UTF-8",
		"Content-Transfer-Encoding": "quoted-printable",
	} {
		if got := msg.Header.Get(name); got != value {
			t.Errorf("%s: %q, want %q", name, got, value)
		}
	}
	if id := msg.Header.Get("Message-Id"); !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@craigdevjohnson.com>") {
		t.Errorf("Message-ID: %q", id)
	}
	if date, err := msg.Header.Date(); err != nil || time.Since(date) > time.Minute {
		t.Errorf("Date: %q (%v)", msg.Header.Get("Date"), err)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(msg.Body))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != e.Body {
		t.Errorf("body:\n%s\nwant:\n%s", body, e.Body)
	}
}

// A server's refusal can quote the Member's address, so what the error says
// is the stage and the reply code alone.
func TestARefusalNamesTheStageAndCodeOnly(t *testing.T) {
	f := newFakeSMTP(t, "550 5.1.1 <friend@example.com>: Recipient address rejected", "221 2.0.0 Bye")
	err := f.sender().Send(context.Background(), New(base, "friend@example.com", nil, nil))
	var e *Error
	if !errors.As(err, &e) || e.Stage != "rcpt" || e.Code != 550 {
		t.Fatalf("got %#v", err)
	}
	if err.Error() != "invite email: rcpt: 550" {
		t.Errorf("error says %q", err.Error())
	}
}

// The server takes the message when it accepts the data, so a QUIT that
// fails afterwards still counts as sent.
func TestAFailedQuitStillCountsAsSent(t *testing.T) {
	f := newFakeSMTP(t, "250 2.1.5 Ok", "421 4.4.2 Timeout")
	if err := f.sender().Send(context.Background(), New(base, "friend@example.com", nil, nil)); err != nil {
		t.Fatalf("got %v", err)
	}
	<-f.done
	if len(f.data) == 0 {
		t.Error("the server took no message")
	}
}

// A header value with a line break is refused before anything is read or
// sent, so it can't add headers or recipients.
func TestAHeaderWithALineBreakIsRefused(t *testing.T) {
	s := &Sender{
		From: from,
		Login: func(context.Context) (string, string, error) {
			t.Error("the login was read for a message that can't be sent")
			return "", "", nil
		},
		Addr: "127.0.0.1:1",
	}
	err := s.Send(context.Background(), Email{To: "a@example.com\r\nBcc: b@example.com", Subject: Subject, Body: "x"})
	var e *Error
	if !errors.As(err, &e) || e.Stage != "message" {
		t.Fatalf("got %#v", err)
	}
}

// When the login can't be read nothing is dialled, and the error says why.
func TestALoginThatCanNotBeReadSendsNothing(t *testing.T) {
	s := &Sender{
		From:  from,
		Login: func(context.Context) (string, string, error) { return "", "", errors.New("parameter missing") },
		Addr:  "127.0.0.1:1",
	}
	err := s.Send(context.Background(), New(base, "a@example.com", nil, nil))
	var e *Error
	if !errors.As(err, &e) || e.Stage != "settings" || !strings.Contains(err.Error(), "parameter missing") {
		t.Fatalf("got %v", err)
	}
}

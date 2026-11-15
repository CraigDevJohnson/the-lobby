package invite

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

// Fastmail is the SMTP server the invite goes through: Fastmail's
// submission port with TLS from the first byte (ADR 0014).
const Fastmail = "smtp.fastmail.com:465"

// wait bounds one whole SMTP exchange, from dialling to QUIT. Lobby Admin's
// add waits for it, inside the 20 seconds the site gives that route.
const wait = 10 * time.Second

// Sender sends invites over SMTP.
type Sender struct {
	// From is the invite's From header, such as
	// "The Lobby <lobby@craigdevjohnson.com>" (INVITE_FROM).
	From string
	// Login gives the SMTP username and password, read when the first
	// invite is sent.
	Login func(ctx context.Context) (username, password string, err error)
	// Addr is the server's host and port; empty means Fastmail.
	Addr string
	// TLS is the client's TLS settings; nil checks the server's certificate
	// against Addr's host, as it should be outside a test.
	TLS *tls.Config
}

// Error is an invite that didn't send: the stage it stopped at (settings,
// message, dial, auth, mail, rcpt or data) and, when the server refused,
// its reply code. A server's refusal can quote the Member's address, or
// the login's, so the error says only those two, except at settings and
// dial, before anything is sent, where the cause can't name anyone.
type Error struct {
	Stage string
	Code  int
	err   error
}

func (e *Error) Error() string {
	switch {
	case e.Code != 0:
		return fmt.Sprintf("invite email: %s: %d", e.Stage, e.Code)
	case e.Stage == "settings" || e.Stage == "dial":
		return "invite email: " + e.Stage + ": " + e.err.Error()
	}
	return "invite email: " + e.Stage
}

func failed(stage string, err error) *Error {
	e := &Error{Stage: stage, err: err}
	var reply *textproto.Error
	if errors.As(err, &reply) {
		e.Code = reply.Code
	}
	return e
}

// Send sends e in one SMTP exchange: TLS, PLAIN auth, MAIL FROM The Lobby's
// address, RCPT TO the Member, DATA, then QUIT. It gives up after about ten
// seconds, or sooner when ctx ends first. Any error is an *Error.
func (s *Sender) Send(ctx context.Context, e Email) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return failed("message", errors.New("the From address can't be read"))
	}
	msg, err := message(s.From, from.Address, e)
	if err != nil {
		return failed("message", err)
	}
	username, password, err := s.Login(ctx)
	if err != nil {
		return failed("settings", err)
	}
	addr := s.Addr
	if addr == "" {
		addr = Fastmail
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return failed("dial", err)
	}
	deadline := time.Now().Add(wait)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Deadline: deadline}, Config: s.TLS}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return failed("dial", err)
	}
	_ = conn.SetDeadline(deadline)
	c, err := smtp.NewClient(conn, host)
	if err != nil {
		conn.Close()
		return failed("dial", err)
	}
	defer c.Close()
	if err := c.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
		return failed("auth", err)
	}
	if err := c.Mail(from.Address); err != nil {
		return failed("mail", err)
	}
	if err := c.Rcpt(e.To); err != nil {
		return failed("rcpt", err)
	}
	w, err := c.Data()
	if err != nil {
		return failed("data", err)
	}
	if _, err := w.Write(msg); err != nil {
		return failed("data", err)
	}
	if err := w.Close(); err != nil {
		return failed("data", err)
	}
	// The server took the message when it accepted the data, so a QUIT that
	// fails doesn't make it unsent.
	_ = c.Quit()
	return nil
}

// message is e as the DATA of an SMTP exchange: its headers, then its body
// as quoted-printable UTF-8. A header value holding a line break could add
// headers or recipients of its own, so one is refused.
func message(fromHeader, fromAddress string, e Email) ([]byte, error) {
	id := make([]byte, 16)
	_, _ = rand.Read(id)
	domain := fromAddress[strings.LastIndex(fromAddress, "@")+1:]
	headers := [][2]string{
		{"From", fromHeader},
		{"To", e.To},
		{"Subject", e.Subject},
		{"Date", time.Now().Format(time.RFC1123Z)},
		{"Message-ID", "<" + hex.EncodeToString(id) + "@" + domain + ">"},
		{"MIME-Version", "1.0"},
		{"Content-Type", "text/plain; charset=UTF-8"},
		{"Content-Transfer-Encoding", "quoted-printable"},
	}
	var b bytes.Buffer
	for _, h := range headers {
		if strings.ContainsAny(h[1], "\r\n") {
			return nil, errors.New("the " + h[0] + " header holds a line break")
		}
		b.WriteString(h[0] + ": " + h[1] + "\r\n")
	}
	b.WriteString("\r\n")
	qp := quotedprintable.NewWriter(&b)
	if _, err := qp.Write([]byte(e.Body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

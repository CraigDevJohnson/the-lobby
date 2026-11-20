// Package invite is the email Lobby Admin sends a new Member (ADR 0014):
// how to sign in, the Tools they were given, and how to reach a Minecraft
// world or Foundry when they hold those grants. It is plain text, sent
// through Fastmail's SMTP server from The Lobby's own address (send.go).
//
// Only Lobby Admin's add sends an invite. A later change to a Member's
// Tools sends nothing, and so does the Owner command.
//
// The sender here also carries the site's one other email: with the daily
// iCloud update on, a Member is told when the league moves a game in their
// iCloud calendar (internal/soccer, ADR 0017).
package invite

import (
	"net"
	"slices"
	"strconv"
	"strings"
)

// Subject is the invite's subject line, kept to ASCII so it needs no
// encoding in the header.
const Subject = "You're invited to The Lobby"

// tool is one Tool as the email names it: its grant, its display name and
// its path. The list is web/src/tools.ts, in its order, which a test keeps
// in step.
type tool struct{ grant, name, path string }

var tools = []tool{
	{"soccer", "Let’s Plan Soccer", "/soccer"},
	{"minecraft", "Minecraft Launcher", "/minecraft"},
	{"foundry", "Foundry", "/foundry"},
	{"minecraft-admin", "Minecraft Admin", "/minecraft/admin"},
	{"foundry-admin", "Foundry Admin", "/foundry/admin"},
	{"lobby-admin", "Lobby Admin", "/lobby/admin"},
}

// Let's Plan Soccer is public, so every Member sees it whatever their
// grants, and the email lists it for everyone.
const alwaysShown = "soccer"

// World is one Minecraft world's name and the addresses a player adds in
// Minecraft, as the Launcher's backend gives them. JavaPort is 0 when the
// world uses Java's default; a Bedrock address always comes with its port.
// A world may have only one of the two.
type World struct {
	Name           string
	JavaAddress    string
	JavaPort       int
	BedrockAddress string
	BedrockPort    int
}

// Email is one email, ready to send: an invite, or the daily iCloud
// update's email about moved games.
type Email struct {
	To      string
	Subject string
	Body    string
}

// New builds the invite for the Member at to. base is the site's own address
// without a trailing slash (SITE_BASE_URL), so each environment's email
// leads to that environment. grants are the Member's Tools; any the email
// doesn't know are left out. worlds are the Minecraft worlds' addresses,
// used only with the minecraft grant; when there are none, because the
// backend couldn't be read, the email links to the Launcher instead.
func New(base, to string, grants []string, worlds []World) Email {
	var b strings.Builder
	line := func(s ...string) {
		b.WriteString(strings.Join(s, ""))
		b.WriteString("\n")
	}
	line("Craig has added you to The Lobby.")
	line()
	line("Sign in")
	line("1. Go to ", base, " and choose Sign in.")
	line("2. Enter ", to, ".")
	line("3. Cloudflare emails you a one-time code. Enter it to finish.")
	line("You stay signed in on that device for up to 90 days.")
	line()
	line("Your Tools")
	for _, t := range tools {
		if t.grant == alwaysShown || slices.Contains(grants, t.grant) {
			line("- ", t.name, ": ", base, t.path)
		}
	}
	if slices.Contains(grants, "minecraft") {
		line()
		line("Minecraft")
		if len(worlds) == 0 {
			line("Start a world on the Minecraft Launcher page, then add it in Minecraft. The page shows each world’s address:")
			line(base, "/minecraft")
		} else {
			line("Start a world on the Minecraft Launcher page, then add it in Minecraft:")
			for _, w := range worlds {
				line("- ", w.Name, ": ", w.join())
			}
		}
		line("Craig adds your player name to the allowlist before you can join.")
	}
	if slices.Contains(grants, "foundry") {
		line()
		line("Foundry")
		line("Check that Foundry is up on its page, then choose Open Foundry and finish joining there.")
	}
	line()
	line("Questions? Reply to this email.")
	return Email{To: to, Subject: Subject, Body: b.String()}
}

// join is what a player adds for w: the Java address, with the Bedrock port
// after it when Bedrock shares the address, or each on its own.
func (w World) join() string {
	java := w.JavaAddress
	if java != "" && w.JavaPort != 0 {
		java = net.JoinHostPort(java, strconv.Itoa(w.JavaPort))
	}
	port := strconv.Itoa(w.BedrockPort)
	switch {
	case w.BedrockAddress == "":
		return java
	case java == "":
		return w.BedrockAddress + ", port " + port + " (Bedrock)"
	case w.BedrockAddress == w.JavaAddress:
		return java + " (Bedrock: port " + port + ")"
	}
	return java + " (Bedrock: " + w.BedrockAddress + ", port " + port + ")"
}

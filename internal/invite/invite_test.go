package invite

import (
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const base = "https://next.example.com"

// A Member with the Launcher and Foundry gets the whole email: how to sign
// in at this environment's address, their Tools with Let's Plan Soccer
// first, each world's addresses, and how to join Foundry.
func TestTheInviteSaysHowToSignInAndWhatTheyCanOpen(t *testing.T) {
	worlds := []World{
		{Name: "Survival", JavaAddress: "survival.mc.example.com", BedrockAddress: "survival.mc.example.com", BedrockPort: 19132},
		{Name: "Creative", JavaAddress: "creative.mc.example.com", BedrockAddress: "creative.mc.example.com", BedrockPort: 19132},
	}
	got := New(base, "friend@example.com", []string{"minecraft", "foundry"}, worlds)
	want := `Craig has added you to The Lobby.

Sign in
1. Go to https://next.example.com and choose Sign in.
2. Enter friend@example.com.
3. Cloudflare emails you a one-time code. Enter it to finish.
You stay signed in on that device for up to 90 days.

Your Tools
- Let’s Plan Soccer: https://next.example.com/soccer
- Minecraft Launcher: https://next.example.com/minecraft
- Foundry: https://next.example.com/foundry

Minecraft
Start a world on the Minecraft Launcher page, then add it in Minecraft:
- Survival: survival.mc.example.com (Bedrock: port 19132)
- Creative: creative.mc.example.com (Bedrock: port 19132)
Craig adds your player name to the allowlist before you can join.

Foundry
Check that Foundry is up on its page, then choose Open Foundry and finish joining there.

Questions? Reply to this email.
`
	if got.Body != want {
		t.Errorf("body:\n%s\nwant:\n%s", got.Body, want)
	}
	if got.To != "friend@example.com" || got.Subject != "You're invited to The Lobby" {
		t.Errorf("to %q, subject %q", got.To, got.Subject)
	}
}

// Every Member is told about Let's Plan Soccer, each admin grant is one
// line in Lobby Admin's order, and the Minecraft and Foundry sections come
// only with their own grants. A grant the email doesn't know is left out.
func TestTheInviteListsOnlyTheirTools(t *testing.T) {
	got := New(base, "a@example.com", []string{"lobby-admin", "retired-tool", "foundry-admin", "minecraft-admin"}, nil).Body
	want := "Your Tools\n" +
		"- Let’s Plan Soccer: https://next.example.com/soccer\n" +
		"- Minecraft Admin: https://next.example.com/minecraft/admin\n" +
		"- Foundry Admin: https://next.example.com/foundry/admin\n" +
		"- Lobby Admin: https://next.example.com/lobby/admin\n" +
		"\nQuestions? Reply to this email.\n"
	if !strings.HasSuffix(got, want) {
		t.Errorf("body:\n%s\nwant it to end:\n%s", got, want)
	}
	for _, absent := range []string{"Minecraft Launcher", "\nMinecraft\n", "\nFoundry\n", "retired-tool", "allowlist"} {
		if strings.Contains(got, absent) {
			t.Errorf("%q in:\n%s", absent, got)
		}
	}
}

// Without the worlds' addresses, because the backend couldn't be read, the
// Minecraft section links to the Launcher, which shows them.
func TestTheInviteLinksToTheLauncherWithoutTheWorlds(t *testing.T) {
	got := New(base, "a@example.com", []string{"minecraft"}, nil).Body
	want := "Minecraft\nStart a world on the Minecraft Launcher page, then add it in Minecraft. The page shows each world’s address:\nhttps://next.example.com/minecraft\nCraig adds your player name to the allowlist before you can join.\n"
	if !strings.Contains(got, want) {
		t.Errorf("body:\n%s", got)
	}
}

// Each world says what a player adds: Java's address with a port only when
// it isn't the default, and Bedrock's port, with its own address when it
// differs or Java has none.
func TestEachWorldSaysWhatToAdd(t *testing.T) {
	for _, c := range []struct {
		world World
		want  string
	}{
		{World{JavaAddress: "mc.example.com", BedrockAddress: "mc.example.com", BedrockPort: 19132}, "mc.example.com (Bedrock: port 19132)"},
		{World{JavaAddress: "mc.example.com", JavaPort: 25566, BedrockAddress: "mc.example.com", BedrockPort: 19133}, "mc.example.com:25566 (Bedrock: port 19133)"},
		{World{JavaAddress: "java.example.com", BedrockAddress: "be.example.com", BedrockPort: 19132}, "java.example.com (Bedrock: be.example.com, port 19132)"},
		{World{JavaAddress: "java.example.com"}, "java.example.com"},
		{World{BedrockAddress: "be.example.com", BedrockPort: 19132}, "be.example.com, port 19132 (Bedrock)"},
	} {
		if got := c.world.join(); got != c.want {
			t.Errorf("%+v: %q, want %q", c.world, got, c.want)
		}
	}
}

// The email names each Tool as The VIP Lobby does, with the same link, in
// the same order: the entries in web/src/tools.ts.
func TestToolsMatchThePagesTools(t *testing.T) {
	src, err := os.ReadFile("../../web/src/tools.ts")
	if err != nil {
		t.Fatal(err)
	}
	var page []tool
	for _, m := range regexp.MustCompile(`\{ id: "([a-z-]+)", name: "([^"]+)",[^}]*href: "([^"]+)"`).FindAllStringSubmatch(string(src), -1) {
		page = append(page, tool{m[1], m[2], m[3]})
	}
	if !slices.Equal(page, tools) {
		t.Errorf("web/src/tools.ts has %v; the email names %v", page, tools)
	}
}

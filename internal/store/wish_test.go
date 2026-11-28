package store

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
)

const (
	listA = "0123456789abcdef0123456789abcdef"
	listB = "fedcba9876543210fedcba9876543210"
	wish1 = "11111111111111111111111111111111"
)

func editor(email, generation string, need ...string) WishEditor {
	return WishEditor{Email: email, Generation: generation, Need: need}
}

func ids(lists []WishList) []string {
	out := []string{}
	for _, l := range lists {
		out = append(out, l.Kind+":"+l.ID)
	}
	slices.Sort(out)
	return out
}

// A list is created at revision 0 and saved over the revision read, each
// save one revision on. A retry of a save that went, or of a create, finds
// the list moved on and changes nothing. A personal list is always the
// editor's own, one for each membership, whatever ID the save names.
func TestWishListsSaveOverTheRevisionRead(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	_ = m.PutMember(ctx, Member{Email: "kid@example.com", Tools: []string{"wish"}, Generation: "first"})
	kid := editor("Kid@example.com", "first", "wish")

	saved, err := m.SaveWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Name: "Kid", Wishes: []Wish{{ID: wish1, Title: "Kite"}}, Owner: "someone@example.com"})
	if err != nil || saved.Revision != 1 || saved.Owner != "kid@example.com" || saved.OwnerGeneration != "first" {
		t.Fatalf("create: %+v, %v", saved, err)
	}
	if _, err := m.SaveWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Name: "Kid"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("the create again: %v, want ErrConflict", err)
	}
	if _, err := m.SaveWishList(ctx, kid, WishList{ID: listB, Kind: WishPersonal, Name: "Second"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("a second personal list: %v, want ErrConflict", err)
	}
	next, err := m.SaveWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Name: "Kid's", Revision: 1})
	if err != nil || next.Revision != 2 || next.Name != "Kid's" || next.Wishes == nil || len(next.Wishes) != 0 {
		t.Fatalf("save: %+v, %v", next, err)
	}
	for name, l := range map[string]WishList{
		"the same revision again": {ID: listA, Kind: WishPersonal, Name: "Old", Revision: 1},
		"a newer revision":        {ID: listA, Kind: WishPersonal, Name: "Ahead", Revision: 3},
		"another ID":              {ID: listB, Kind: WishPersonal, Name: "Other", Revision: 2},
		"the other kind":          {ID: listA, Kind: WishManaged, Name: "Managed", Revision: 2},
	} {
		need := kid
		if l.Kind == WishManaged {
			_ = m.PutMember(ctx, Member{Email: "kid@example.com", Tools: []string{"wish", "lobby-admin"}, Generation: "first"})
			need = editor("kid@example.com", "first", "wish", "lobby-admin")
		}
		if _, err := m.SaveWishList(ctx, need, l); !errors.Is(err, ErrConflict) {
			t.Errorf("%s: %v, want ErrConflict", name, err)
		}
	}
	if err := m.DeleteWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Revision: 1}); !errors.Is(err, ErrConflict) {
		t.Errorf("delete at an old revision: %v, want ErrConflict", err)
	}
	if err := m.DeleteWishList(ctx, kid, WishList{ID: listB, Kind: WishPersonal, Revision: 2}); !errors.Is(err, ErrConflict) {
		t.Errorf("delete of another ID: %v, want ErrConflict", err)
	}
	lists, _ := m.ListWishLists(ctx)
	if len(lists) != 1 || lists[0].Name != "Kid's" || lists[0].Revision != 2 {
		t.Fatalf("after the refused writes: %+v", lists)
	}
	// What a caller is given back is a copy.
	lists[0].Wishes = append(lists[0].Wishes, Wish{ID: wish1, Title: "Changed"})
	if again, _ := m.ListWishLists(ctx); len(again[0].Wishes) != 0 {
		t.Fatalf("a caller changed what is stored: %+v", again)
	}
	if err := m.DeleteWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Revision: 2}); err != nil {
		t.Fatal(err)
	}
	if err := m.DeleteWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Revision: 2}); !errors.Is(err, ErrConflict) {
		t.Errorf("the delete again: %v, want ErrConflict", err)
	}
	if lists, _ := m.ListWishLists(ctx); len(lists) != 0 {
		t.Fatalf("after the delete: %+v", lists)
	}
}

// A write holds only while the editor is still the Member the request
// checked, with every grant it needs: not removed, not being removed, not
// invited again since, and not without a grant taken away since the check.
// Each refusal changes nothing.
func TestWishListWritesNeedTheCheckedMembership(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	_ = m.PutMember(ctx, Member{Email: "owner@example.com", Tools: []string{"wish", "lobby-admin"}, Generation: "first"})
	_ = m.PutWishList(ctx, WishList{ID: listB, Kind: WishManaged, Name: "Grandpa", Revision: 4})
	managed := WishList{ID: listB, Kind: WishManaged, Name: "Grandpa", Revision: 4}
	for name, by := range map[string]WishEditor{
		"another generation":    editor("owner@example.com", "second", "wish", "lobby-admin"),
		"before generations":    editor("owner@example.com", "", "wish", "lobby-admin"),
		"a grant it never held": editor("owner@example.com", "first", "wish", "lobby-admin", "minecraft"),
		"nobody":                editor("stranger@example.com", "first", "wish"),
	} {
		if _, err := m.SaveWishList(ctx, by, managed); !errors.Is(err, ErrAccessChanged) {
			t.Errorf("save by %s: %v, want ErrAccessChanged", name, err)
		}
		if err := m.DeleteWishList(ctx, by, managed); !errors.Is(err, ErrAccessChanged) {
			t.Errorf("delete by %s: %v, want ErrAccessChanged", name, err)
		}
	}
	owner := editor("owner@example.com", "first", "wish", "lobby-admin")
	// The grant taken away after the check, before the write.
	_ = m.ReplaceMember(ctx, Member{Email: "owner@example.com", Tools: []string{"wish"}, Generation: "first"})
	if _, err := m.SaveWishList(ctx, owner, managed); !errors.Is(err, ErrAccessChanged) {
		t.Errorf("without lobby-admin: %v", err)
	}
	_ = m.ReplaceMember(ctx, Member{Email: "owner@example.com", Tools: []string{"wish", "lobby-admin"}, Generation: "first"})
	m.FailCleanup(errors.New("stopped"))
	_ = m.DeleteMember(ctx, "owner@example.com")
	if _, err := m.SaveWishList(ctx, owner, managed); !errors.Is(err, ErrAccessChanged) {
		t.Errorf("being removed: %v", err)
	}
	lists, _ := m.ListWishLists(ctx)
	if len(lists) != 1 || lists[0].Revision != 4 {
		t.Fatalf("a refused write changed the list: %+v", lists)
	}
}

// The family sees every managed list and each live membership's personal
// list. A removal ends a personal list's showing as it begins, and deletes it
// before the Member's record goes, retried until it does. Managed lists stay.
// A list left from an earlier invitation of the address never shows beside a
// new one, and the new invitation's own list is untouched by the old one's
// writes or a removal of it.
func TestWishListsFollowTheMembershipThatMadeThem(t *testing.T) {
	ctx := context.Background()
	m := NewMemory()
	_ = m.PutMember(ctx, Member{Email: "kid@example.com", Tools: []string{"wish"}, Generation: "first"})
	_ = m.PutMember(ctx, Member{Email: "old@example.com", Tools: []string{}})
	kid := editor("kid@example.com", "first", "wish")
	if _, err := m.SaveWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Name: "Kid"}); err != nil {
		t.Fatal(err)
	}
	_ = m.PutWishList(ctx, WishList{ID: listB, Kind: WishManaged, Name: "Baby", Revision: 1})
	_ = m.PutWishList(ctx, WishList{ID: wish1, Kind: WishPersonal, Name: "Legacy", Revision: 1, Owner: "Old@example.com"})
	if lists, _ := m.ListWishLists(ctx); !slices.Equal(ids(lists), []string{"managed:" + listB, "personal:" + listA, "personal:" + wish1}) {
		t.Fatalf("the family sees %v", ids(lists))
	}

	m.FailCleanup(errors.New("stopped"))
	if err := m.DeleteMember(ctx, "kid@example.com"); err == nil {
		t.Fatal("the removal didn't stop")
	}
	if lists, _ := m.ListWishLists(ctx); !slices.Equal(ids(lists), []string{"managed:" + listB, "personal:" + wish1}) {
		t.Fatalf("while the removal is unfinished the family sees %v", ids(lists))
	}
	if !m.HasWishList(WishPersonal, listA, "kid@example.com", "first") {
		t.Fatal("the stopped removal deleted the list")
	}
	m.FailCleanup(nil)
	if err := m.DeleteMember(ctx, "kid@example.com"); err != nil {
		t.Fatal(err)
	}
	if m.HasWishList(WishPersonal, listA, "kid@example.com", "first") {
		t.Fatal("the removal left the personal list")
	}
	if err := m.DeleteMember(ctx, "old@example.com"); err != nil {
		t.Fatal(err)
	}
	if lists, _ := m.ListWishLists(ctx); !slices.Equal(ids(lists), []string{"managed:" + listB}) {
		t.Fatalf("after the removals the family sees %v", ids(lists))
	}

	// Invited again: the earlier membership's writes are refused, and the
	// new one's list is its own.
	_ = m.CreateMember(ctx, Member{Email: "kid@example.com", Tools: []string{"wish"}, Generation: "second"})
	if _, err := m.SaveWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Name: "Kid"}); !errors.Is(err, ErrAccessChanged) {
		t.Fatalf("the earlier membership's create: %v", err)
	}
	again := editor("kid@example.com", "second", "wish")
	if _, err := m.SaveWishList(ctx, again, WishList{ID: listA, Kind: WishPersonal, Name: "Kid again"}); err != nil {
		t.Fatal(err)
	}
	// A list kept from the earlier membership never shows beside it.
	_ = m.PutWishList(ctx, WishList{ID: wish1, Kind: WishPersonal, Name: "Stale", Revision: 1, Owner: "kid@example.com", OwnerGeneration: "first"})
	lists, _ := m.ListWishLists(ctx)
	if !slices.Equal(ids(lists), []string{"managed:" + listB, "personal:" + listA}) || lists[0].Name != "Kid again" && lists[1].Name != "Kid again" {
		t.Fatalf("after the new invitation the family sees %+v", lists)
	}
	if err := m.DeleteWishList(ctx, kid, WishList{ID: listA, Kind: WishPersonal, Revision: 1}); !errors.Is(err, ErrAccessChanged) {
		t.Fatalf("the earlier membership's delete: %v", err)
	}
	if !m.HasWishList(WishPersonal, listA, "kid@example.com", "second") {
		t.Fatal("the new invitation's list is gone")
	}
}

// A wish list write is one transaction: an Update of the editor's Member
// item, which sets its kind to what it is, held to the membership the
// request checked and every grant the change needs, beside the list's own
// write, held to the revision read. A membership from before generations is
// held to having none. Each action names only what its expressions use.
func TestWishListWritesAreOneConditionalTransaction(t *testing.T) {
	status, body := http.StatusOK, `{}`
	var ops []string
	d, bodies := fakeTable(t, func(operation string) (int, string) {
		ops = append(ops, operation)
		return status, body
	})
	ctx := context.Background()
	type action struct {
		TableName                 string
		Key                       map[string]map[string]string
		Item                      map[string]json.RawMessage
		UpdateExpression          string
		ConditionExpression       string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]string
	}
	sent := func() (action, map[string]action) {
		t.Helper()
		if ops[len(ops)-1] != "TransactWriteItems" {
			t.Fatalf("sent %v", ops)
		}
		var tx struct{ TransactItems []map[string]action }
		if err := json.Unmarshal([]byte((*bodies)[len(*bodies)-1]), &tx); err != nil {
			t.Fatal(err)
		}
		if len(tx.TransactItems) != 2 {
			t.Fatalf("sent %s", (*bodies)[len(*bodies)-1])
		}
		return tx.TransactItems[0]["Update"], tx.TransactItems[1]
	}
	str := func(raw json.RawMessage) string {
		var v map[string]any
		_ = json.Unmarshal(raw, &v)
		if s, ok := v["S"].(string); ok {
			return s
		}
		if n, ok := v["N"].(string); ok {
			return "N" + n
		}
		return string(raw)
	}

	saved, err := d.SaveWishList(ctx, editor("Kid@example.com", "first", "wish"), WishList{ID: listA, Kind: WishPersonal, Name: "Kid", Wishes: []Wish{{ID: wish1, Title: "Kite", URL: "https://example.com/kite"}}})
	if err != nil || saved.Revision != 1 || saved.Owner != "kid@example.com" {
		t.Fatalf("create: %+v, %v", saved, err)
	}
	member, write := sent()
	if member.TableName != "site-test-sessions" || member.Key["id"]["S"] != "member#kid@example.com" || member.UpdateExpression != "SET #kind = :member" ||
		member.ConditionExpression != "#generation = :generation AND attribute_not_exists(#removing) AND contains(#tools, :grant0)" ||
		len(member.ExpressionAttributeNames) != 4 || member.ExpressionAttributeNames["#tools"] != "tools" || member.ExpressionAttributeNames["#generation"] != "generation" ||
		len(member.ExpressionAttributeValues) != 3 || member.ExpressionAttributeValues[":generation"]["S"] != "first" || member.ExpressionAttributeValues[":grant0"]["S"] != "wish" {
		t.Fatalf("the Member's condition sent %s", (*bodies)[len(*bodies)-1])
	}
	put := write["Put"]
	if put.TableName != "site-test-sessions" || str(put.Item["id"]) != "wish#personal#first#kid@example.com" || str(put.Item["kind"]) != "wish" ||
		str(put.Item["list_id"]) != listA || str(put.Item["list_kind"]) != "personal" || str(put.Item["revision"]) != "N1" ||
		str(put.Item["owner"]) != "kid@example.com" || str(put.Item["owner_generation"]) != "first" ||
		put.ConditionExpression != "attribute_not_exists(#id)" || len(put.ExpressionAttributeNames) != 1 || len(put.ExpressionAttributeValues) != 0 {
		t.Fatalf("the create sent %s", (*bodies)[len(*bodies)-1])
	}
	if !strings.Contains(string(put.Item["wishes"]), `"url":{"S":"https://example.com/kite"}`) || strings.Contains(string(put.Item["wishes"]), `"notes"`) {
		t.Errorf("the wishes sent %s", put.Item["wishes"])
	}

	// The Owner's change to a managed list, held to the revision read.
	saved, err = d.SaveWishList(ctx, editor("owner@example.com", "", "wish", "lobby-admin"), WishList{ID: listB, Kind: WishManaged, Name: "Baby", Revision: 3})
	if err != nil || saved.Revision != 4 || saved.Owner != "" || saved.Wishes == nil {
		t.Fatalf("save: %+v, %v", saved, err)
	}
	member, write = sent()
	if member.Key["id"]["S"] != "member#owner@example.com" ||
		member.ConditionExpression != "attribute_exists(#id) AND attribute_not_exists(#generation) AND attribute_not_exists(#removing) AND contains(#tools, :grant0) AND contains(#tools, :grant1)" ||
		len(member.ExpressionAttributeNames) != 5 || len(member.ExpressionAttributeValues) != 3 ||
		member.ExpressionAttributeValues[":grant0"]["S"] != "wish" || member.ExpressionAttributeValues[":grant1"]["S"] != "lobby-admin" {
		t.Fatalf("the legacy Owner's condition sent %s", (*bodies)[len(*bodies)-1])
	}
	put = write["Put"]
	if str(put.Item["id"]) != "wish#managed#"+listB || str(put.Item["revision"]) != "N4" || put.Item["owner"] != nil || put.Item["owner_generation"] != nil ||
		string(put.Item["wishes"]) != `{"L":[]}` ||
		put.ConditionExpression != "#list = :list AND #revision = :revision" || put.ExpressionAttributeNames["#list"] != "list_id" || put.ExpressionAttributeNames["#revision"] != "revision" ||
		put.ExpressionAttributeValues[":list"]["S"] != listB || put.ExpressionAttributeValues[":revision"]["N"] != "3" {
		t.Fatalf("the save sent %s", (*bodies)[len(*bodies)-1])
	}

	if err := d.DeleteWishList(ctx, editor("kid@example.com", "first", "wish"), WishList{ID: listA, Kind: WishPersonal, Revision: 2}); err != nil {
		t.Fatal(err)
	}
	_, write = sent()
	if del := write["Delete"]; del.Key["id"]["S"] != "wish#personal#first#kid@example.com" || del.ConditionExpression != "#list = :list AND #revision = :revision" ||
		del.ExpressionAttributeValues[":list"]["S"] != listA || del.ExpressionAttributeValues[":revision"]["N"] != "2" {
		t.Fatalf("the delete sent %s", (*bodies)[len(*bodies)-1])
	}
	n := len(*bodies)
	if err := d.DeleteWishList(ctx, editor("kid@example.com", "first", "wish"), WishList{ID: listA, Kind: WishPersonal}); !errors.Is(err, ErrConflict) || len(*bodies) != n {
		t.Fatalf("a delete at revision 0: %v, sent %d", err, len(*bodies)-n)
	}
	if _, err := d.SaveWishList(ctx, editor("kid@example.com", "first", "wish"), WishList{ID: listA, Kind: "shared"}); err == nil || len(*bodies) != n {
		t.Fatalf("an unknown kind: %v, sent %d", err, len(*bodies)-n)
	}

	// DynamoDB cancels the transaction and gives a reason for each action
	// in order: the Member's refused is ErrAccessChanged, before anything
	// the list's says, and the list's is ErrConflict. Anything else is a
	// failure that names only the kind of record.
	canceled := func(member, list string) string {
		return `{"__type":"com.amazonaws.dynamodb.v20120810#TransactionCanceledException","Message":"Transaction cancelled","CancellationReasons":[{"Code":"` + member + `"},{"Code":"` + list + `"}]}`
	}
	status = http.StatusBadRequest
	for _, c := range []struct {
		body string
		want error
	}{
		{canceled("ConditionalCheckFailed", "None"), ErrAccessChanged},
		{canceled("ConditionalCheckFailed", "ConditionalCheckFailed"), ErrAccessChanged},
		{canceled("None", "ConditionalCheckFailed"), ErrConflict},
	} {
		body = c.body
		if _, err := d.SaveWishList(ctx, editor("kid@example.com", "first", "wish"), WishList{ID: listA, Kind: WishPersonal, Name: "Kid", Revision: 1}); !errors.Is(err, c.want) {
			t.Errorf("%s: save %v, want %v", c.body, err, c.want)
		}
		if err := d.DeleteWishList(ctx, editor("kid@example.com", "first", "wish"), WishList{ID: listA, Kind: WishPersonal, Revision: 1}); !errors.Is(err, c.want) {
			t.Errorf("%s: delete %v, want %v", c.body, err, c.want)
		}
	}
	for _, failure := range []string{canceled("None", "TransactionConflict"), `{"__type":"com.amazonaws.dynamodb.v20120810#ResourceNotFoundException","message":"Requested resource not found"}`} {
		body = failure
		_, err := d.SaveWishList(ctx, editor("kid@example.com", "first", "wish"), WishList{ID: listA, Kind: WishPersonal, Name: "Kid", Revision: 1})
		if err == nil || errors.Is(err, ErrConflict) || errors.Is(err, ErrAccessChanged) || strings.Contains(err.Error(), "kid@") || strings.Contains(err.Error(), listA) || !strings.Contains(err.Error(), "save wish:") {
			t.Errorf("%s: %v", failure, err)
		}
	}
}

// The family's lists come from one strongly consistent scan of Members and
// lists: each managed list, and each personal list whose Member is there, is
// not being removed and has the list's generation, the empty one included.
func TestListWishListsShowsLiveMembershipsLists(t *testing.T) {
	const page = `{"Count":9,"ScannedCount":12,"Items":[
		{"id":{"S":"member#kid@example.com"},"kind":{"S":"member"},"email":{"S":"kid@example.com"},"tools":{"L":[{"S":"wish"}]},"added_at":{"S":"2026-10-05T12:00:00Z"},"generation":{"S":"second"}},
		{"id":{"S":"member#going@example.com"},"kind":{"S":"member"},"email":{"S":"going@example.com"},"tools":{"L":[]},"added_at":{"S":"2026-10-05T12:00:00Z"},"generation":{"S":"g"},"removing":{"BOOL":true}},
		{"id":{"S":"member#legacy@example.com"},"kind":{"S":"member"},"email":{"S":"legacy@example.com"},"tools":{"L":[]},"added_at":{"S":"2026-10-05T12:00:00Z"}},
		{"id":{"S":"wish#personal#second#kid@example.com"},"kind":{"S":"wish"},"list_id":{"S":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},"list_kind":{"S":"personal"},"name":{"S":"Kid"},"revision":{"N":"2"},"wishes":{"L":[{"M":{"id":{"S":"11111111111111111111111111111111"},"title":{"S":"Kite"},"notes":{"S":"red"}}}]},"owner":{"S":"kid@example.com"},"owner_generation":{"S":"second"}},
		{"id":{"S":"wish#personal#first#kid@example.com"},"kind":{"S":"wish"},"list_id":{"S":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"list_kind":{"S":"personal"},"name":{"S":"Stale"},"revision":{"N":"1"},"wishes":{"L":[]},"owner":{"S":"kid@example.com"},"owner_generation":{"S":"first"}},
		{"id":{"S":"wish#personal#g#going@example.com"},"kind":{"S":"wish"},"list_id":{"S":"cccccccccccccccccccccccccccccccc"},"list_kind":{"S":"personal"},"name":{"S":"Going"},"revision":{"N":"1"},"wishes":{"L":[]},"owner":{"S":"going@example.com"},"owner_generation":{"S":"g"}},
		{"id":{"S":"wish#personal##legacy@example.com"},"kind":{"S":"wish"},"list_id":{"S":"dddddddddddddddddddddddddddddddd"},"list_kind":{"S":"personal"},"name":{"S":"Legacy"},"revision":{"N":"1"},"wishes":{"L":[]},"owner":{"S":"legacy@example.com"}},
		{"id":{"S":"wish#personal#x#gone@example.com"},"kind":{"S":"wish"},"list_id":{"S":"eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"},"list_kind":{"S":"personal"},"name":{"S":"Gone"},"revision":{"N":"1"},"wishes":{"L":[]},"owner":{"S":"gone@example.com"},"owner_generation":{"S":"x"}},
		{"id":{"S":"wish#managed#ffffffffffffffffffffffffffffffff"},"kind":{"S":"wish"},"list_id":{"S":"ffffffffffffffffffffffffffffffff"},"list_kind":{"S":"managed"},"name":{"S":"Baby"},"revision":{"N":"5"},"wishes":{"L":[]}}
	]}`
	var ops []string
	d, bodies := fakeTable(t, func(operation string) (int, string) {
		ops = append(ops, operation)
		return http.StatusOK, page
	})
	lists, err := d.ListWishLists(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(lists); !slices.Equal(got, []string{"managed:ffffffffffffffffffffffffffffffff", "personal:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "personal:dddddddddddddddddddddddddddddddd"}) {
		t.Fatalf("lists %v", got)
	}
	for _, l := range lists {
		if l.ID == "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" && (l.Name != "Kid" || l.Revision != 2 || len(l.Wishes) != 1 || l.Wishes[0] != (Wish{ID: wish1, Title: "Kite", Notes: "red"}) || l.Owner != "kid@example.com" || l.OwnerGeneration != "second") {
			t.Errorf("the Member's list read as %+v", l)
		}
	}
	var scan struct {
		ConsistentRead            bool
		FilterExpression          string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]string
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &scan); err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0] != "Scan" || !scan.ConsistentRead || scan.FilterExpression != "#kind = :member OR #kind = :wish" ||
		scan.ExpressionAttributeValues[":member"]["S"] != "member" || scan.ExpressionAttributeValues[":wish"]["S"] != "wish" {
		t.Fatalf("sent %v %s", ops, (*bodies)[0])
	}

	d, _ = fakeTable(t, func(string) (int, string) { return http.StatusInternalServerError, serverError })
	if _, err := d.ListWishLists(context.Background()); err == nil || !strings.Contains(err.Error(), "list wish:") {
		t.Fatalf("a failed scan: %v", err)
	}
}

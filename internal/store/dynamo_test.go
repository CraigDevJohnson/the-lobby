package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// A failed read is reported without its key: a session's key is the sign-in
// cookie's value and a Member's is their address, and the error goes to the
// log.
func TestAFailedReadKeepsItsKeyOutOfTheError(t *testing.T) {
	failing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"__type":"com.amazonaws.dynamodb.v20120810#ResourceNotFoundException","message":"Requested resource not found"}`))
	}))
	t.Cleanup(failing.Close)
	client := dynamodb.New(dynamodb.Options{
		Region:           "us-west-2",
		BaseEndpoint:     aws.String(failing.URL),
		Credentials:      credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", ""),
		RetryMaxAttempts: 1,
	})
	d := NewDynamo(client, "site-test-sessions")

	const cookie = "c2Vzc2lvbi1jb29raWUtdmFsdWU"
	_, err := d.GetSession(context.Background(), cookie)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("GetSession: %v, want the read's own failure", err)
	}
	if strings.Contains(err.Error(), cookie) {
		t.Errorf("the error carries the cookie's value: %v", err)
	}
	if !strings.Contains(err.Error(), "session") {
		t.Errorf("the error doesn't say what was being read: %v", err)
	}

	const address = "player@example.com"
	_, err = d.GetMember(context.Background(), address)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("GetMember: %v, want the read's own failure", err)
	}
	if strings.Contains(err.Error(), address) {
		t.Errorf("the error carries the Member's address: %v", err)
	}
	if !strings.Contains(err.Error(), "member") {
		t.Errorf("the error doesn't say what was being read: %v", err)
	}

	const secret = "bWVtYmVyLWxpbmstc2VjcmV0LW1lbWJlci1saW5rLXM"
	_, err = d.SoccerLink(context.Background(), secret)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("SoccerLink: %v, want the read's own failure", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the error carries the Member link's secret: %v", err)
	}
	if !strings.Contains(err.Error(), "soccerlink") {
		t.Errorf("the error doesn't say what was being read: %v", err)
	}
}

// fakeTable answers each DynamoDB call with what answer gives for its
// operation, and keeps the calls' bodies.
func fakeTable(t *testing.T, answer func(operation string) (int, string)) (*Dynamo, *[]string) {
	t.Helper()
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		status, body := answer(strings.TrimPrefix(r.Header.Get("X-Amz-Target"), "DynamoDB_20120810."))
		w.Header().Set("Content-Type", "application/x-amz-json-1.0")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	client := dynamodb.New(dynamodb.Options{
		Region:           "us-west-2",
		BaseEndpoint:     aws.String(srv.URL),
		Credentials:      credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", ""),
		RetryMaxAttempts: 1,
	})
	return NewDynamo(client, "site-test-sessions"), &bodies
}

// sentRequests is each call fakeTable received, as its operation and the key
// it named, in the order they came. ops is the operations the answer function
// was given.
func sentRequests(t *testing.T, ops, bodies []string) []string {
	t.Helper()
	if len(ops) != len(bodies) {
		t.Fatalf("%d operations, %d bodies", len(ops), len(bodies))
	}
	out := make([]string, 0, len(bodies))
	for i, body := range bodies {
		var sent struct {
			Key map[string]map[string]string
		}
		if err := json.Unmarshal([]byte(body), &sent); err != nil {
			t.Fatal(err)
		}
		out = append(out, ops[i]+" "+sent.Key["id"]["S"])
	}
	return out
}

const serverError = `{"__type":"com.amazonaws.dynamodb.v20120810#InternalServerError","message":"failed"}`

// A removal marks the Member's item before it deletes anything, and deletes
// that item only after every record that could reconnect an old calendar or
// Member link has gone. If the mark fails, nothing is deleted. If any later
// step fails, the marked item remains: it ends the Member's access, blocks
// adding the address again and lets Remove be tried again.
func TestDeleteMemberMarksTheMemberFirstAndDeletesThemLast(t *testing.T) {
	const soccer = `{"Item":{"id":{"S":"soccer#player@example.com"},"kind":{"S":"soccer"},"email":{"S":"player@example.com"},"teams":{"L":[]},"secret":{"S":"s1"}}}`
	var ops []string
	d, bodies := fakeTable(t, func(operation string) (int, string) {
		ops = append(ops, operation)
		if operation == "GetItem" {
			return http.StatusOK, soccer
		}
		return http.StatusOK, `{}`
	})
	if err := d.DeleteMember(context.Background(), "Player@example.com"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"UpdateItem member#player@example.com",
		"GetItem soccer#player@example.com",
		"DeleteItem soccerlink#s1",
		"DeleteItem soccer#player@example.com",
		"DeleteItem google#player@example.com",
		"DeleteItem icloud#player@example.com",
		"DeleteItem member#player@example.com",
	}
	if got := sentRequests(t, ops, *bodies); !slices.Equal(got, want) {
		t.Fatalf("requests %v, want %v", got, want)
	}
	// The mark sets one attribute, and only on an item that is there: an
	// update with no condition would make an item for an address with none.
	var mark struct {
		UpdateExpression          string
		ConditionExpression       string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]any
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &mark); err != nil {
		t.Fatal(err)
	}
	if mark.UpdateExpression != "SET #removing = :removing" || mark.ConditionExpression != "attribute_exists(#id)" ||
		len(mark.ExpressionAttributeNames) != 2 || mark.ExpressionAttributeNames["#removing"] != "removing" || mark.ExpressionAttributeNames["#id"] != "id" ||
		len(mark.ExpressionAttributeValues) != 1 || mark.ExpressionAttributeValues[":removing"]["BOOL"] != true {
		t.Fatalf("the mark sent %s", (*bodies)[0])
	}

	// Each step failing in turn, the mark first: nothing after it is sent,
	// so the Member's item is deleted only when everything before it went.
	for failAt, step := range want {
		t.Run(step, func(t *testing.T) {
			var ops []string
			d, bodies := fakeTable(t, func(operation string) (int, string) {
				ops = append(ops, operation)
				switch {
				case len(ops) == failAt+1:
					return http.StatusInternalServerError, serverError
				case operation == "GetItem":
					return http.StatusOK, soccer
				}
				return http.StatusOK, `{}`
			})
			err := d.DeleteMember(context.Background(), "player@example.com")
			if err == nil || strings.Contains(err.Error(), "player@") {
				t.Fatalf("a failed %s: %v", step, err)
			}
			if got := sentRequests(t, ops, *bodies); !slices.Equal(got, want[:failAt+1]) {
				t.Fatalf("requests %v, want %v", got, want[:failAt+1])
			}
		})
	}
}

// A Member without Remembered teams still has its connected calendars and
// record removed, with the Member last. An address with no Member item has
// nothing to mark, and what is left of its calendar is deleted all the same.
func TestDeleteMemberWithoutSoccerOrWithoutTheMember(t *testing.T) {
	want := []string{
		"UpdateItem member#player@example.com",
		"GetItem soccer#player@example.com",
		"DeleteItem google#player@example.com",
		"DeleteItem icloud#player@example.com",
		"DeleteItem member#player@example.com",
	}
	for name, mark := range map[string]struct {
		status int
		body   string
	}{
		"no Remembered teams": {http.StatusOK, `{}`},
		"no Member item":      {http.StatusBadRequest, `{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"The conditional request failed"}`},
	} {
		var ops []string
		d, bodies := fakeTable(t, func(operation string) (int, string) {
			ops = append(ops, operation)
			if operation == "UpdateItem" {
				return mark.status, mark.body
			}
			return http.StatusOK, `{}`
		})
		if err := d.DeleteMember(context.Background(), "Player@example.com"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := sentRequests(t, ops, *bodies); !slices.Equal(got, want) {
			t.Fatalf("%s: requests %v, want %v", name, got, want)
		}
	}
}

// A Member whose item carries a removal's mark is read as ErrRemoving, which
// is an ErrNotFound: everything that checks access reads the Member here. An
// item written before the mark, or before generations, has neither attribute
// and reads as the Member it is.
func TestGetMemberBeingRemoved(t *testing.T) {
	const item = `{"Item":{"id":{"S":"member#player@example.com"},"kind":{"S":"member"},"email":{"S":"player@example.com"},"tools":{"L":[{"S":"soccer"}]},"added_at":{"S":"2026-10-05T12:00:00Z"}%s}}`
	answer := ``
	d, _ := fakeTable(t, func(string) (int, string) { return http.StatusOK, strings.Replace(item, "%s", answer, 1) })
	ctx := context.Background()

	m, err := d.GetMember(ctx, "player@example.com")
	if err != nil || m.Email != "player@example.com" || m.Generation != "" || !slices.Equal(m.Tools, []string{"soccer"}) {
		t.Fatalf("a Member from before the mark: %+v, %v", m, err)
	}
	answer = `,"generation":{"S":"first"},"removing":{"BOOL":true}`
	m, err = d.GetMember(ctx, "player@example.com")
	if !errors.Is(err, ErrRemoving) || !errors.Is(err, ErrNotFound) || m.Email != "" || m.Tools != nil {
		t.Fatalf("a Member being removed: %+v, %v", m, err)
	}
	// The Owner's list still shows them, to be removed again.
	answer = `{"Count":1,"ScannedCount":1,"Items":[{"id":{"S":"member#player@example.com"},"kind":{"S":"member"},"email":{"S":"player@example.com"},"tools":{"L":[]},"added_at":{"S":"2026-10-05T12:00:00Z"},"removing":{"BOOL":true}}]}`
	d, _ = fakeTable(t, func(string) (int, string) { return http.StatusOK, answer })
	if all, err := d.ListMembers(ctx); err != nil || len(all) != 1 || all[0].Email != "player@example.com" {
		t.Fatalf("the list: %+v, %v", all, err)
	}
}

// The records a removal deletes are written only for an address with a
// Member item that no removal has marked: each write is one transaction that
// holds that condition on the Member's item beside the put, so DynamoDB
// applies both or neither. The condition is on an Update that sets the
// Member item's kind to what it is, which the site role may do; a
// ConditionCheck needs a permission it doesn't have. The condition names
// neither a generation nor anything else an item made before these had, so
// such a Member's calendar is still written.
func TestCalendarWritesNeedAMemberWhoIsNotBeingRemoved(t *testing.T) {
	status, body := http.StatusOK, `{}`
	var ops []string
	d, bodies := fakeTable(t, func(operation string) (int, string) {
		ops = append(ops, operation)
		return status, body
	})
	ctx := context.Background()
	writes := []struct {
		kind, key string
		put       func() error
	}{
		{"google", "google#player@example.com", func() error {
			return d.PutGoogle(ctx, Google{Email: "Player@example.com", Token: "sealed-token", CalendarID: "primary", ConnectionID: "google-1"})
		}},
		{"icloud", "icloud#player@example.com", func() error {
			return d.PutICloud(ctx, ICloud{Email: "Player@example.com", Password: "sealed-password", ConnectionID: "icloud-1"})
		}},
	}
	type action struct {
		TableName                 string
		Key                       map[string]map[string]string
		Item                      map[string]map[string]any
		UpdateExpression          string
		ConditionExpression       *string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]string
	}
	for i, w := range writes {
		if err := w.put(); err != nil {
			t.Fatalf("%s: %v", w.kind, err)
		}
		if len(*bodies) != i+1 || ops[i] != "TransactWriteItems" {
			t.Fatalf("%s: sent %v", w.kind, ops)
		}
		var sent struct {
			TransactItems []map[string]action
		}
		if err := json.Unmarshal([]byte((*bodies)[i]), &sent); err != nil {
			t.Fatal(err)
		}
		if len(sent.TransactItems) != 2 || len(sent.TransactItems[0]) != 1 || len(sent.TransactItems[1]) != 1 {
			t.Fatalf("%s sent %s", w.kind, (*bodies)[i])
		}
		member, put := sent.TransactItems[0]["Update"], sent.TransactItems[1]["Put"]
		if member.TableName != "site-test-sessions" || member.Key["id"]["S"] != "member#player@example.com" ||
			member.ConditionExpression == nil || *member.ConditionExpression != "attribute_exists(#id) AND attribute_not_exists(#removing)" ||
			member.UpdateExpression != "SET #kind = :member" || len(member.ExpressionAttributeNames) != 3 || member.ExpressionAttributeNames["#id"] != "id" ||
			member.ExpressionAttributeNames["#removing"] != "removing" || member.ExpressionAttributeNames["#kind"] != "kind" ||
			len(member.ExpressionAttributeValues) != 1 || member.ExpressionAttributeValues[":member"]["S"] != "member" {
			t.Fatalf("%s: the Member's condition sent %s", w.kind, (*bodies)[i])
		}
		if put.TableName != "site-test-sessions" || put.Item["id"]["S"] != w.key || put.Item["kind"]["S"] != w.kind || put.Item["email"]["S"] != "player@example.com" || put.ConditionExpression != nil {
			t.Fatalf("%s: the put sent %s", w.kind, (*bodies)[i])
		}
	}

	// DynamoDB cancels the whole transaction and says why for each action in
	// order. The Member's condition refused is ErrNotFound: no Member, or one
	// being removed, and nothing was written. Anything else is a failure that
	// names only the kind of record.
	canceled := func(member string) string {
		return `{"__type":"com.amazonaws.dynamodb.v20120810#TransactionCanceledException","Message":"Transaction cancelled, please refer cancellation reasons for specific reasons [` + member + `, None]","CancellationReasons":[{"Code":"` + member + `","Message":"refused"},{"Code":"None"}]}`
	}
	for _, w := range writes {
		status, body = http.StatusBadRequest, canceled("ConditionalCheckFailed")
		if err := w.put(); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s with no Member, or one being removed: %v, want ErrNotFound", w.kind, err)
		}
		for name, failure := range map[string]string{
			"another write to the Member at the same moment": canceled("TransactionConflict"),
			"a failed write": `{"__type":"com.amazonaws.dynamodb.v20120810#ResourceNotFoundException","message":"Requested resource not found"}`,
		} {
			body = failure
			err := w.put()
			if err == nil || errors.Is(err, ErrNotFound) || strings.Contains(err.Error(), "player@") || !strings.Contains(err.Error(), "put "+w.kind+":") {
				t.Errorf("%s, %s: %v", w.kind, name, err)
			}
		}
	}
}

func TestRenewSessionIsConditional(t *testing.T) {
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	current := time.Date(2027, 1, 4, 12, 0, 0, 0, time.UTC)
	expires := current.Add(48 * time.Hour)
	if err := d.RenewSession(context.Background(), "cookie", current, expires); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Key                       map[string]map[string]string
		UpdateExpression          string
		ConditionExpression       string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]string
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil {
		t.Fatal(err)
	}
	if sent.Key["id"]["S"] != "session#cookie" || sent.UpdateExpression != "SET #expires = :expires" ||
		sent.ConditionExpression != "#expires = :current" || sent.ExpressionAttributeNames["#expires"] != "expires_at" ||
		sent.ExpressionAttributeValues[":current"]["N"] != "1799064000" || sent.ExpressionAttributeValues[":expires"]["N"] != "1799236800" {
		t.Fatalf("sent %s", (*bodies)[0])
	}

	// GetSession treats a session past its expiry by the real clock as gone,
	// so the session another renewal advanced expires from that clock. The
	// fixed timestamps above are only for the wire format.
	seen := time.Now().Add(24 * time.Hour)
	advanced := seen.Add(48 * time.Hour)
	conditionFailed := `{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"The conditional request failed"}`
	d, _ = fakeTable(t, func(operation string) (int, string) {
		if operation == "UpdateItem" {
			return http.StatusBadRequest, conditionFailed
		}
		return http.StatusOK, fmt.Sprintf(`{"Item":{"id":{"S":"session#cookie"},"kind":{"S":"session"},"email":{"S":"craig@example.com"},"created_at":{"S":"2026-10-06T12:00:00Z"},"expires_at":{"N":"%d"}}}`, advanced.Unix())
	})
	if err := d.RenewSession(context.Background(), "cookie", seen, advanced); err != nil {
		t.Fatalf("another renewal won: %v", err)
	}

	d, _ = fakeTable(t, func(operation string) (int, string) {
		if operation == "UpdateItem" {
			return http.StatusBadRequest, conditionFailed
		}
		return http.StatusOK, `{}`
	})
	if err := d.RenewSession(context.Background(), "cookie", current, expires); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleted session: %v, want ErrNotFound", err)
	}
}

// A Member's item is written only over what was read: a new Member where
// the address has no item, and a change over an item that still has the
// generation read, or still has none when it was written before generations,
// and that no removal has marked. DynamoDB refusing the condition (the
// address was added, removed or invited again since, or is being removed) is
// ErrConflict, and any other failure keeps the Member's address out. Each
// call names only what its condition uses, since DynamoDB refuses a name or
// a value that no expression uses.
func TestMemberWritesAreConditional(t *testing.T) {
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	ctx := context.Background()
	type put struct {
		Item                      map[string]map[string]any
		ConditionExpression       string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]string
	}
	last := func() put {
		t.Helper()
		var sent put
		if err := json.Unmarshal([]byte((*bodies)[len(*bodies)-1]), &sent); err != nil {
			t.Fatal(err)
		}
		return sent
	}
	m := Member{Email: "Player@example.com", Tools: []string{"soccer"}, AddedAt: time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC), Generation: "first"}

	if err := d.CreateMember(ctx, m); err != nil {
		t.Fatal(err)
	}
	if sent := last(); sent.Item["id"]["S"] != "member#player@example.com" || sent.Item["generation"]["S"] != "first" || sent.Item["added_at"]["S"] != "2026-11-20T12:00:00Z" ||
		sent.ConditionExpression != "attribute_not_exists(#id)" || len(sent.ExpressionAttributeNames) != 1 || sent.ExpressionAttributeNames["#id"] != "id" || len(sent.ExpressionAttributeValues) != 0 {
		t.Fatalf("a new Member sent %s", (*bodies)[len(*bodies)-1])
	}

	if err := d.ReplaceMember(ctx, m); err != nil {
		t.Fatal(err)
	}
	if sent := last(); sent.Item["id"]["S"] != "member#player@example.com" || sent.Item["generation"]["S"] != "first" ||
		sent.ConditionExpression != "#generation = :generation AND attribute_not_exists(#removing)" || len(sent.ExpressionAttributeNames) != 2 ||
		sent.ExpressionAttributeNames["#generation"] != "generation" || sent.ExpressionAttributeNames["#removing"] != "removing" ||
		len(sent.ExpressionAttributeValues) != 1 || sent.ExpressionAttributeValues[":generation"]["S"] != "first" {
		t.Fatalf("a change sent %s", (*bodies)[len(*bodies)-1])
	}

	before := m
	before.Generation = ""
	if err := d.ReplaceMember(ctx, before); err != nil {
		t.Fatal(err)
	}
	sent := last()
	if sent.Item["id"]["S"] != "member#player@example.com" || sent.ConditionExpression != "attribute_exists(#id) AND attribute_not_exists(#generation) AND attribute_not_exists(#removing)" ||
		len(sent.ExpressionAttributeNames) != 3 || sent.ExpressionAttributeNames["#id"] != "id" || sent.ExpressionAttributeNames["#generation"] != "generation" ||
		sent.ExpressionAttributeNames["#removing"] != "removing" || len(sent.ExpressionAttributeValues) != 0 {
		t.Fatalf("a change to a Member from before generations sent %s", (*bodies)[len(*bodies)-1])
	}
	if _, has := sent.Item["generation"]; has {
		t.Errorf("a Member from before generations was given one: %s", (*bodies)[len(*bodies)-1])
	}
	// Only a removal writes its mark: a Member's item is never put with one.
	for _, body := range *bodies {
		if strings.Contains(body, `"removing":{`) {
			t.Errorf("a Member write carries the removal's mark: %s", body)
		}
	}

	status, body = http.StatusBadRequest, `{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"The conditional request failed"}`
	if err := d.CreateMember(ctx, m); !errors.Is(err, ErrConflict) {
		t.Errorf("a new Member, condition refused: %v, want ErrConflict", err)
	}
	if err := d.ReplaceMember(ctx, m); !errors.Is(err, ErrConflict) {
		t.Errorf("a change, condition refused: %v, want ErrConflict", err)
	}
	if err := d.ReplaceMember(ctx, before); !errors.Is(err, ErrConflict) {
		t.Errorf("a change to a Member from before generations, condition refused: %v, want ErrConflict", err)
	}
	body = `{"__type":"com.amazonaws.dynamodb.v20120810#ResourceNotFoundException","message":"Requested resource not found"}`
	for _, err := range []error{d.CreateMember(ctx, m), d.ReplaceMember(ctx, m)} {
		if err == nil || errors.Is(err, ErrConflict) || strings.Contains(err.Error(), "player@") || !strings.Contains(err.Error(), "member") {
			t.Errorf("a failed write: %v", err)
		}
	}
}

// A settings change names only the mutable fields and is conditioned on the
// immutable connection ID, sealed credential and settings snapshot. The
// provider-error deletes use the same connection condition, so neither kind
// of operation can update, recreate or delete a newer connection.
func TestCalendarConnectionMutationsAreConditionalAndNarrow(t *testing.T) {
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	ctx := context.Background()
	g := Google{Email: "Player@example.com", Account: "player@gmail.example", Token: "sealed-token", CalendarID: "primary", ConnectionID: "google-1"}
	c := ICloud{Email: "Player@example.com", Account: "player@icloud.example", Password: "sealed-password", Home: "home", CalendarID: "family", ConnectionID: "icloud-1", Daily: true,
		Written: []string{"game-101@soccer.example"}, Times: map[string]GameTime{"game-101@soccer.example": {Start: "20261121T191500", Field: "Field 3"}}}
	if err := d.UpdateGoogleCalendar(ctx, g, "family"); err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateICloudCalendar(ctx, c, "home"); err != nil {
		t.Fatal(err)
	}
	if err := d.UpdateICloudDailySwitch(ctx, c, true, "home"); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteGoogleConnection(ctx, g); err != nil {
		t.Fatal(err)
	}
	if err := d.DeleteICloudConnection(ctx, c); err != nil {
		t.Fatal(err)
	}
	if len(*bodies) != 5 {
		t.Fatalf("sent %d requests", len(*bodies))
	}
	type sentMutation struct {
		Key                       map[string]map[string]string
		UpdateExpression          string
		ConditionExpression       string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]any
	}
	read := func(at int) sentMutation {
		var sent sentMutation
		if err := json.Unmarshal([]byte((*bodies)[at]), &sent); err != nil {
			t.Fatal(err)
		}
		return sent
	}
	googleUpdate := read(0)
	if googleUpdate.Key["id"]["S"] != "google#player@example.com" || googleUpdate.UpdateExpression != "SET #calendar = :calendar, #written = :written, #times = :times ADD #revision :one" ||
		googleUpdate.ConditionExpression != "#connection = :connection AND #credential = :credential AND #calendar = :current_calendar AND (attribute_not_exists(#daily) OR #daily = :current_daily)" ||
		googleUpdate.ExpressionAttributeNames["#credential"] != "token" || googleUpdate.ExpressionAttributeValues[":connection"]["S"] != "google-1" ||
		googleUpdate.ExpressionAttributeValues[":credential"]["S"] != "sealed-token" || googleUpdate.ExpressionAttributeValues[":current_calendar"]["S"] != "primary" || googleUpdate.ExpressionAttributeValues[":calendar"]["S"] != "family" {
		t.Fatalf("Google update sent %s", (*bodies)[0])
	}
	icloudUpdate := read(1)
	if icloudUpdate.Key["id"]["S"] != "icloud#player@example.com" || icloudUpdate.UpdateExpression != "SET #calendar = :calendar" ||
		icloudUpdate.ConditionExpression != "#connection = :connection AND #credential = :credential AND #calendar = :current_calendar AND #daily = :current_daily" ||
		icloudUpdate.ExpressionAttributeNames["#credential"] != "password" || icloudUpdate.ExpressionAttributeValues[":connection"]["S"] != "icloud-1" ||
		icloudUpdate.ExpressionAttributeValues[":credential"]["S"] != "sealed-password" || icloudUpdate.ExpressionAttributeValues[":current_calendar"]["S"] != "family" || icloudUpdate.ExpressionAttributeValues[":calendar"]["S"] != "family" {
		t.Fatalf("iCloud calendar update sent %s", (*bodies)[1])
	}
	switchUpdate := read(2)
	if switchUpdate.UpdateExpression != "SET #daily = :daily, #calendar = :calendar, #written = :written, #times = :times" ||
		switchUpdate.ConditionExpression != "#connection = :connection AND #credential = :credential AND #calendar = :current_calendar AND #daily = :current_daily" ||
		switchUpdate.ExpressionAttributeValues[":current_daily"]["BOOL"] != true || switchUpdate.ExpressionAttributeValues[":daily"]["BOOL"] != true {
		t.Fatalf("iCloud switch update sent %s", (*bodies)[2])
	}
	for at, want := range []struct {
		index      int
		key        string
		credential string
		connection string
	}{{3, "google#player@example.com", "sealed-token", "google-1"}, {4, "icloud#player@example.com", "sealed-password", "icloud-1"}} {
		sent := read(want.index)
		if sent.Key["id"]["S"] != want.key || sent.UpdateExpression != "" || sent.ConditionExpression != "#connection = :connection AND #credential = :credential" ||
			sent.ExpressionAttributeValues[":credential"]["S"] != want.credential || sent.ExpressionAttributeValues[":connection"]["S"] != want.connection {
			t.Fatalf("conditional delete %d sent %s", at, (*bodies)[want.index])
		}
	}

	// A pre-generation record remains mutable only while its credential
	// record still has no connection_id. A missing item or new-generation
	// item therefore cannot satisfy this condition.
	legacy := Google{Email: g.Email, Token: g.Token, CalendarID: g.CalendarID}
	if err := d.UpdateGoogleCalendar(ctx, legacy, "family"); err != nil {
		t.Fatal(err)
	}
	legacyUpdate := read(5)
	if legacyUpdate.ConditionExpression != "attribute_not_exists(#connection) AND #credential = :credential AND #calendar = :current_calendar AND (attribute_not_exists(#daily) OR #daily = :current_daily)" {
		t.Fatalf("legacy update sent %s", (*bodies)[5])
	}

	status, body = http.StatusBadRequest, `{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"The conditional request failed"}`
	if err := d.UpdateGoogleCalendar(ctx, g, "family"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("condition refused: %v, want ErrNotFound", err)
	}
}

// The daily iCloud update's save names only the switch, the list and each
// game's time and field, and only for the connection it read, with the daily
// update still on: DynamoDB refusing the condition (the Member disconnected,
// connected again, picked another calendar or turned the switch off) is
// ErrNotFound, and any other failure keeps the Member's address out.
func TestUpdateICloudDailyIsConditional(t *testing.T) {
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	c := ICloud{Email: "Player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", ConnectionID: "icloud-1", Daily: true,
		Written: []string{"game-101@soccer.example"}, Times: map[string]GameTime{"game-101@soccer.example": {Start: "20261121T191500", Field: "Field 3"}}}
	if err := d.UpdateICloudDaily(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Key                       map[string]map[string]string
		UpdateExpression          string
		ConditionExpression       string
		ExpressionAttributeNames  map[string]string
		ExpressionAttributeValues map[string]map[string]any
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil {
		t.Fatal(err)
	}
	names, values := sent.ExpressionAttributeNames, sent.ExpressionAttributeValues
	if sent.Key["id"]["S"] != "icloud#player@example.com" ||
		sent.UpdateExpression != "SET #daily = :daily, #written = :written, #times = :times" || names["#daily"] != "daily" || names["#written"] != "written" || names["#times"] != "times" ||
		sent.ConditionExpression != "#connection = :connection AND #credential = :credential AND #calendar = :calendar AND #daily = :on" || values[":on"]["BOOL"] != true || names["#connection"] != "connection_id" || names["#credential"] != "password" || names["#calendar"] != "calendar_id" ||
		values[":connection"]["S"] != "icloud-1" || values[":credential"]["S"] != "sealed" || values[":calendar"]["S"] != "family" || values[":daily"]["BOOL"] != true {
		t.Fatalf("sent %s", (*bodies)[0])
	}
	// All three attributes are set: the list, and the map of each game's
	// start and field by UID.
	if written, _ := values[":written"]["L"].([]any); len(written) != 1 {
		t.Errorf("written sent as %v", values[":written"])
	}
	if got := sentTimes(values[":times"]); len(got) != 1 || got["game-101@soccer.example"] != (GameTime{Start: "20261121T191500", Field: "Field 3"}) {
		t.Errorf("times sent as %v", values[":times"])
	}
	for _, never := range []string{"player@icloud.example", `"home"`} {
		if strings.Contains((*bodies)[0], never) {
			t.Errorf("the update carries %s: %s", never, (*bodies)[0])
		}
	}

	status, body = http.StatusBadRequest, `{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException","message":"The conditional request failed"}`
	if err := d.UpdateICloudDaily(context.Background(), c); !errors.Is(err, ErrNotFound) {
		t.Fatalf("condition refused: %v, want ErrNotFound", err)
	}
	body = `{"__type":"com.amazonaws.dynamodb.v20120810#ResourceNotFoundException","message":"Requested resource not found"}`
	err := d.UpdateICloudDaily(context.Background(), c)
	if err == nil || errors.Is(err, ErrNotFound) || strings.Contains(err.Error(), "player@") || !strings.Contains(err.Error(), "icloud") {
		t.Fatalf("a failed update: %v", err)
	}
}

// Saving a newly authenticated connection carries every connection field,
// including its immutable ID. Settings changes use the narrow methods above,
// but PutICloud still has a complete representation of the record.
func TestPutICloudCarriesTheSwitchAndTheList(t *testing.T) {
	d, bodies := fakeTable(t, func(string) (int, string) { return http.StatusOK, `{}` })
	c := ICloud{Email: "Player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", ConnectionID: "icloud-1", Daily: true,
		Written: []string{"game-101@soccer.example"}, Times: map[string]GameTime{"game-101@soccer.example": {Start: "20261121T191500", Field: "Field 3"}}}
	if err := d.PutICloud(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	// The put is the second action of the transaction, after the Member's
	// condition.
	var sent struct {
		TransactItems []struct {
			Put struct {
				Item map[string]map[string]any
			}
		}
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil || len(sent.TransactItems) != 2 {
		t.Fatalf("sent %s, %v", (*bodies)[0], err)
	}
	item := sent.TransactItems[1].Put.Item
	written, _ := item["written"]["L"].([]any)
	if item["id"]["S"] != "icloud#player@example.com" || item["connection_id"]["S"] != "icloud-1" || item["daily"]["BOOL"] != true || len(written) != 1 {
		t.Fatalf("sent %s", (*bodies)[0])
	}
	if got := sentTimes(item["times"]); len(got) != 1 || got["game-101@soccer.example"] != (GameTime{Start: "20261121T191500", Field: "Field 3"}) {
		t.Fatalf("times sent as %v", item["times"])
	}
}

// sentTimes reads the times attribute as it went to DynamoDB: a map by UID
// of maps with a start and a field.
func sentTimes(attribute map[string]any) map[string]GameTime {
	out := map[string]GameTime{}
	byUID, _ := attribute["M"].(map[string]any)
	for uid, v := range byUID {
		entry, _ := v.(map[string]any)
		fields, _ := entry["M"].(map[string]any)
		if len(fields) != 2 {
			continue
		}
		start, _ := fields["start"].(map[string]any)
		field, _ := fields["field"].(map[string]any)
		at := GameTime{}
		at.Start, _ = start["S"].(string)
		at.Field, _ = field["S"].(string)
		out[uid] = at
	}
	return out
}

// Every iCloud connection is listed with its switch, its list and each
// game's time and field. One made before the daily update has none of them,
// and one whose list was written before the times were kept reads with its
// list as it was and a nil map.
func TestListICloud(t *testing.T) {
	d, bodies := fakeTable(t, func(string) (int, string) {
		return http.StatusOK, `{"Count":3,"ScannedCount":9,"Items":[
			{"id":{"S":"icloud#player@example.com"},"kind":{"S":"icloud"},"email":{"S":"player@example.com"},"account":{"S":"player@icloud.example"},"password":{"S":"sealed"},"home":{"S":"home"},"calendar_id":{"S":"family"},"daily":{"BOOL":true},"written":{"L":[{"S":"game-101@soccer.example"}]}},
			{"id":{"S":"icloud#other@example.com"},"kind":{"S":"icloud"},"email":{"S":"other@example.com"},"account":{"S":"other@icloud.example"},"password":{"S":"sealed-2"},"home":{"S":"home-2"},"calendar_id":{"S":""}},
			{"id":{"S":"icloud#zoe@example.com"},"kind":{"S":"icloud"},"email":{"S":"zoe@example.com"},"account":{"S":"zoe@icloud.example"},"password":{"S":"sealed-3"},"home":{"S":"home-3"},"calendar_id":{"S":"family"},"daily":{"BOOL":true},"written":{"L":[{"S":"game-101@soccer.example"},{"S":"session-over-111111@soccer.example"}]},"times":{"M":{"game-101@soccer.example":{"M":{"start":{"S":"20261121T191500"},"field":{"S":"Field 3"}}}}}}]}`
	})
	all, err := d.ListICloud(context.Background())
	if err != nil || len(all) != 3 {
		t.Fatalf("list: %+v, %v", all, err)
	}
	if c := all[0]; c.Email != "player@example.com" || c.Password != "sealed" || c.CalendarID != "family" || !c.Daily || len(c.Written) != 1 || c.Written[0] != "game-101@soccer.example" || c.Times != nil {
		t.Errorf("an item from before the times were kept: %+v", c)
	}
	if c := all[1]; c.Email != "other@example.com" || c.Daily || c.Written != nil || c.Times != nil {
		t.Errorf("second: %+v", c)
	}
	if c := all[2]; len(c.Written) != 2 || len(c.Times) != 1 || c.Times["game-101@soccer.example"] != (GameTime{Start: "20261121T191500", Field: "Field 3"}) {
		t.Errorf("an item with times: %+v", c)
	}
	if !strings.Contains((*bodies)[0], `"FilterExpression":"kind = :k"`) || !strings.Contains((*bodies)[0], `{"S":"icloud"}`) {
		t.Errorf("scan sent %s", (*bodies)[0])
	}
}

// One Member's connection is read the same way: an item from before the
// times were kept gives its list and a nil map, and so does one whose times
// were saved as NULL, as a connection with none is.
func TestGetICloudReadsAnItemFromBeforeTheTimes(t *testing.T) {
	item := `{"Item":{"id":{"S":"icloud#player@example.com"},"kind":{"S":"icloud"},"email":{"S":"player@example.com"},"account":{"S":"player@icloud.example"},"password":{"S":"sealed"},"home":{"S":"home"},"calendar_id":{"S":"family"},"daily":{"BOOL":true},"written":{"L":[{"S":"game-101@soccer.example"},{"S":"game-102@soccer.example"}]}%s}}`
	for name, times := range map[string]string{"no times": ``, "times saved as NULL": `,"times":{"NULL":true}`} {
		d, _ := fakeTable(t, func(string) (int, string) { return http.StatusOK, strings.Replace(item, "%s", times, 1) })
		c, err := d.GetICloud(context.Background(), "player@example.com")
		if err != nil || !c.Daily || len(c.Written) != 2 || c.Written[1] != "game-102@soccer.example" || c.Times != nil {
			t.Errorf("%s: %+v, %v", name, c, err)
		}
	}
}

func TestGoogleDailyConnectionAndLegacyRead(t *testing.T) {
	ctx := context.Background()
	legacy := `{"email":{"S":"old@example.com"},"token":{"S":"sealed"},"calendar_id":{"S":"primary"}}`
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	g := Google{Email: "player@example.com", Account: "player@google.example", Token: "sealed", CalendarID: "family", Daily: true,
		ConnectionID: "connection", DailyRevision: 4, Written: []string{"abc123"}, Times: map[string]GameTime{"abc123": {Start: "20261121T191500", Field: "Field 3"}}}
	if err := d.PutGoogle(ctx, g); err != nil {
		t.Fatal(err)
	}
	var put struct {
		TransactItems []struct {
			Put struct{ Item json.RawMessage }
		}
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &put); err != nil || len(put.TransactItems) != 2 {
		t.Fatalf("put: %v, %s", err, (*bodies)[0])
	}
	item := string(put.TransactItems[1].Put.Item)
	body = `{"Items":[` + legacy + "," + item + "]}"
	listed, err := d.ListGoogle(ctx)
	if err != nil || len(listed) != 2 || listed[0].Daily || listed[0].DailyRevision != 0 || listed[0].Written != nil ||
		listed[1].DailyRevision != 4 || !listed[1].Daily || len(listed[1].Written) != 1 || listed[1].Written[0] != "abc123" || listed[1].Times["abc123"] != g.Times["abc123"] {
		t.Fatalf("list: %+v, %v", listed, err)
	}
	if !strings.Contains((*bodies)[1], `"FilterExpression":"kind = :k"`) || !strings.Contains((*bodies)[1], `{"S":"google"}`) {
		t.Fatalf("scan: %s", (*bodies)[1])
	}
	body = `{"Item":` + item + "}"
	loaded, err := d.GetGoogle(ctx, g.Email)
	if err != nil || loaded.DailyRevision != 4 || !loaded.Daily || len(loaded.Written) != 1 || loaded.Written[0] != "abc123" || loaded.Times["abc123"] != g.Times["abc123"] {
		t.Fatalf("get: %+v, %v", loaded, err)
	}
	body = `{}`
	if err := d.UpdateGoogleDailySwitch(ctx, listed[0], true, "family"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"attribute_not_exists(#connection)", "attribute_not_exists(#daily)", "#credential = :credential", "#calendar = :current_calendar", "ADD #revision :one", `"#revision":"daily_revision"`} {
		if !strings.Contains((*bodies)[3], want) {
			t.Errorf("legacy opt-in lacks %q: %s", want, (*bodies)[3])
		}
	}
	if err := d.UpdateGoogleDaily(ctx, g); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"#connection = :connection AND #credential = :credential AND #calendar = :calendar AND #daily = :on AND #revision = :revision",
		"SET #daily = :daily, #written = :written, #times = :times ADD #revision :one", `"#credential":"token"`, `":revision":{"N":"4"}`, `":one":{"N":"1"}`} {
		if !strings.Contains((*bodies)[4], want) {
			t.Errorf("daily save lacks %q: %s", want, (*bodies)[4])
		}
	}
	status, body = http.StatusBadRequest, `{"__type":"com.amazonaws.dynamodb.v20120810#ConditionalCheckFailedException"}`
	if err := d.UpdateGoogleDaily(ctx, g); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale save: %v", err)
	}
}

func TestDynamoFollowsRequireMemberAndCalendarSnapshot(t *testing.T) {
	ctx := context.Background()
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	was := Soccer{Email: "player@example.com", Teams: []RememberedTeam{{Name: "ROVERS", TeamIDs: []string{"111111"}}}, Secret: "first"}
	next := []RememberedTeam{{Name: "ROVERS", TeamIDs: []string{"222222", "111111"}}}
	if err := d.UpdateSoccerTeams(ctx, was, next); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"member#player@example.com", "attribute_exists(#id) AND attribute_not_exists(#removing)", "soccer#player@example.com",
		"SET #teams = :next", "#teams = :previous AND #secret = :secret", `":secret":{"S":"first"}`, `":previous":{"L":[`, `":next":{"L":[`, "222222"} {
		if !strings.Contains((*bodies)[0], want) {
			t.Errorf("follow lacks %q: %s", want, (*bodies)[0])
		}
	}
	status, body = http.StatusBadRequest, `{"__type":"com.amazonaws.dynamodb.v20120810#TransactionCanceledException","CancellationReasons":[{"Code":"None"},{"Code":"ConditionalCheckFailed"}]}`
	if err := d.UpdateSoccerTeams(ctx, was, next); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
}

func TestDynamoCreatesTeamsAndRevokesOnlyTheLegacyLink(t *testing.T) {
	ctx := context.Background()
	d, bodies := fakeTable(t, func(string) (int, string) { return http.StatusOK, "{}" })
	s := Soccer{Email: "player@example.com"}
	if err := d.UpdateSoccerTeams(ctx, s, []RememberedTeam{{Name: "ROVERS", TeamIDs: []string{"111111"}}}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"attribute_not_exists(#id) OR (#teams = :previous AND (attribute_not_exists(#secret) OR #secret = :secret))",
		"SET #teams = :next, #kind = :kind, #email = :email", `":previous":{"L":[]}`, `":email":{"S":"player@example.com"}`} {
		if !strings.Contains((*bodies)[0], want) {
			t.Errorf("first teams lack %q: %s", want, (*bodies)[0])
		}
	}
	s.Secret = "legacy"
	if err := d.RevokeSoccerLink(ctx, s); err != nil {
		t.Fatal(err)
	}
	var transaction struct {
		TransactItems []map[string]struct {
			Key                                   map[string]map[string]string
			UpdateExpression, ConditionExpression string
		}
	}
	if err := json.Unmarshal([]byte((*bodies)[1]), &transaction); err != nil || len(transaction.TransactItems) != 3 {
		t.Fatalf("revoke: %v, %s", err, (*bodies)[1])
	}
	update := transaction.TransactItems[1]["Update"]
	del := transaction.TransactItems[2]["Delete"]
	if update.Key["id"]["S"] != "soccer#player@example.com" || update.UpdateExpression != "REMOVE #secret" || update.ConditionExpression != "#secret = :secret" ||
		del.Key["id"]["S"] != "soccerlink#legacy" {
		t.Fatalf("revoke: %s", (*bodies)[1])
	}
}

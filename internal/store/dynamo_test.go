package store

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

// A Member is deleted only after every record that could reconnect an old
// calendar or Member link has gone. If any child delete fails, the Member
// remains to block re-adding the address and to let Remove be tried again.
func TestDeleteMemberDeletesTheMemberLast(t *testing.T) {
	const soccer = `{"Item":{"id":{"S":"soccer#player@example.com"},"kind":{"S":"soccer"},"email":{"S":"player@example.com"},"teams":{"L":[]},"secret":{"S":"s1"}}}`
	d, bodies := fakeTable(t, func(operation string) (int, string) {
		if operation == "GetItem" {
			return http.StatusOK, soccer
		}
		return http.StatusOK, `{}`
	})
	if err := d.DeleteMember(context.Background(), "Player@example.com"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"soccer#player@example.com",
		"soccerlink#s1",
		"soccer#player@example.com",
		"google#player@example.com",
		"icloud#player@example.com",
		"member#player@example.com",
	}
	if got := requestKeys(t, *bodies); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("requests %v, want %v", got, want)
	}

	t.Run("soccer read", func(t *testing.T) {
		d, bodies := fakeTable(t, func(string) (int, string) {
			return http.StatusInternalServerError, `{"__type":"com.amazonaws.dynamodb.v20120810#InternalServerError","message":"failed"}`
		})
		if err := d.DeleteMember(context.Background(), "player@example.com"); err == nil {
			t.Fatal("calendar read failure was ignored")
		}
		if got := requestKeys(t, *bodies); len(got) != 1 || got[0] != "soccer#player@example.com" {
			t.Fatalf("requests %v, want only the calendar read", got)
		}
	})

	for failAt, key := range want[1 : len(want)-1] {
		t.Run(key, func(t *testing.T) {
			deletes := 0
			d, bodies := fakeTable(t, func(operation string) (int, string) {
				if operation == "GetItem" {
					return http.StatusOK, soccer
				}
				deletes++
				if deletes == failAt+1 {
					return http.StatusInternalServerError, `{"__type":"com.amazonaws.dynamodb.v20120810#InternalServerError","message":"failed"}`
				}
				return http.StatusOK, `{}`
			})
			if err := d.DeleteMember(context.Background(), "player@example.com"); err == nil {
				t.Fatal("cleanup failure was ignored")
			}
			for _, got := range requestKeys(t, *bodies) {
				if got == "member#player@example.com" {
					t.Fatalf("Member delete sent after %s failed", key)
				}
			}
		})
	}
}

// A Member without Remembered teams still has its connected calendars and
// record removed, with the Member last.
func TestDeleteMemberWithoutSoccerDeletesTheMemberLast(t *testing.T) {
	d, bodies := fakeTable(t, func(operation string) (int, string) {
		if operation == "GetItem" {
			return http.StatusOK, `{}`
		}
		return http.StatusOK, `{}`
	})
	if err := d.DeleteMember(context.Background(), "Player@example.com"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"soccer#player@example.com",
		"google#player@example.com",
		"icloud#player@example.com",
		"member#player@example.com",
	}
	if got := requestKeys(t, *bodies); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("requests %v, want %v", got, want)
	}
}

func requestKeys(t *testing.T, bodies []string) []string {
	t.Helper()
	keys := make([]string, 0, len(bodies))
	for _, body := range bodies {
		var sent struct {
			Key map[string]map[string]string
		}
		if err := json.Unmarshal([]byte(body), &sent); err != nil {
			t.Fatal(err)
		}
		keys = append(keys, sent.Key["id"]["S"])
	}
	return keys
}

// The daily iCloud update's save names only the switch, the list and each
// game's time and field, and only for the connection it read, with the daily
// update still on: DynamoDB refusing the condition (the Member disconnected,
// connected again, picked another calendar or turned the switch off) is
// ErrNotFound, and any other failure keeps the Member's address out.
func TestUpdateICloudDailyIsConditional(t *testing.T) {
	status, body := http.StatusOK, `{}`
	d, bodies := fakeTable(t, func(string) (int, string) { return status, body })
	c := ICloud{Email: "Player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", Daily: true,
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
		sent.ConditionExpression != "#password = :password AND #calendar = :calendar AND #daily = :on" || values[":on"]["BOOL"] != true || names["#password"] != "password" || names["#calendar"] != "calendar_id" ||
		values[":password"]["S"] != "sealed" || values[":calendar"]["S"] != "family" || values[":daily"]["BOOL"] != true {
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

// Saving a connection, as the switch and a press do, carries the daily
// update's switch, its list and each game's time and field with it: the
// item is written whole, so one left out would be wiped.
func TestPutICloudCarriesTheSwitchAndTheList(t *testing.T) {
	d, bodies := fakeTable(t, func(string) (int, string) { return http.StatusOK, `{}` })
	c := ICloud{Email: "Player@example.com", Account: "player@icloud.example", Password: "sealed", Home: "home", CalendarID: "family", Daily: true,
		Written: []string{"game-101@soccer.example"}, Times: map[string]GameTime{"game-101@soccer.example": {Start: "20261121T191500", Field: "Field 3"}}}
	if err := d.PutICloud(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	var sent struct {
		Item map[string]map[string]any
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil {
		t.Fatal(err)
	}
	written, _ := sent.Item["written"]["L"].([]any)
	if sent.Item["id"]["S"] != "icloud#player@example.com" || sent.Item["daily"]["BOOL"] != true || len(written) != 1 {
		t.Fatalf("sent %s", (*bodies)[0])
	}
	if got := sentTimes(sent.Item["times"]); len(got) != 1 || got["game-101@soccer.example"] != (GameTime{Start: "20261121T191500", Field: "Field 3"}) {
		t.Fatalf("times sent as %v", sent.Item["times"])
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

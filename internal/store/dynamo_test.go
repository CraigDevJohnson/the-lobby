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
	if googleUpdate.Key["id"]["S"] != "google#player@example.com" || googleUpdate.UpdateExpression != "SET #calendar = :calendar" ||
		googleUpdate.ConditionExpression != "#connection = :connection AND #credential = :credential AND #calendar = :current_calendar" ||
		googleUpdate.ExpressionAttributeNames["#credential"] != "token" || googleUpdate.ExpressionAttributeValues[":connection"]["S"] != "google-1" ||
		googleUpdate.ExpressionAttributeValues[":credential"]["S"] != "sealed-token" || googleUpdate.ExpressionAttributeValues[":current_calendar"]["S"] != "primary" || googleUpdate.ExpressionAttributeValues[":calendar"]["S"] != "family" {
		t.Fatalf("Google update sent %s", (*bodies)[0])
	}
	icloudUpdate := read(1)
	if icloudUpdate.Key["id"]["S"] != "icloud#player@example.com" || icloudUpdate.UpdateExpression != "SET #calendar = :calendar, #written = :written, #times = :times" ||
		icloudUpdate.ConditionExpression != "#connection = :connection AND #credential = :credential AND #calendar = :current_calendar" ||
		icloudUpdate.ExpressionAttributeNames["#credential"] != "password" || icloudUpdate.ExpressionAttributeValues[":connection"]["S"] != "icloud-1" ||
		icloudUpdate.ExpressionAttributeValues[":credential"]["S"] != "sealed-password" || icloudUpdate.ExpressionAttributeValues[":current_calendar"]["S"] != "family" || icloudUpdate.ExpressionAttributeValues[":calendar"]["S"] != "home" {
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
	if legacyUpdate.ConditionExpression != "attribute_not_exists(#connection) AND #credential = :credential AND #calendar = :current_calendar" {
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
	var sent struct {
		Item map[string]map[string]any
	}
	if err := json.Unmarshal([]byte((*bodies)[0]), &sent); err != nil {
		t.Fatal(err)
	}
	written, _ := sent.Item["written"]["L"].([]any)
	if sent.Item["id"]["S"] != "icloud#player@example.com" || sent.Item["connection_id"]["S"] != "icloud-1" || sent.Item["daily"]["BOOL"] != true || len(written) != 1 {
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

package store

import (
	"context"
	"errors"
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
}

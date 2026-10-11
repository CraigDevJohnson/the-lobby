package backend

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials"
)

// A Function URL with IAM auth refuses a signed request without the hash of
// its body in X-Amz-Content-Sha256, so the signer must send the header and
// sign it.
func TestSignedRequestsCarryTheBodyHash(t *testing.T) {
	sign := SigV4(credentials.NewStaticCredentialsProvider("AKIDEXAMPLE", "secret", "session"), "us-west-2")

	for name, body := range map[string][]byte{"with a body": []byte(`{"team_ids":["123456"]}`), "without a body": nil} {
		t.Run(name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "https://example.lambda-url.us-west-2.on.aws/calendar", bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			if err := sign(req, body); err != nil {
				t.Fatal(err)
			}

			sum := sha256.Sum256(body)
			if got, want := req.Header.Get("X-Amz-Content-Sha256"), hex.EncodeToString(sum[:]); got != want {
				t.Fatalf("X-Amz-Content-Sha256 %q, want %q", got, want)
			}
			auth := req.Header.Get("Authorization")
			if !strings.HasPrefix(auth, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/") || !strings.Contains(auth, "/us-west-2/lambda/aws4_request") {
				t.Fatalf("Authorization %q", auth)
			}
			signed := strings.Split(strings.TrimPrefix(strings.Split(auth, ", ")[1], "SignedHeaders="), ";")
			for _, header := range []string{"host", "x-amz-content-sha256", "x-amz-date", "x-amz-security-token"} {
				found := false
				for _, s := range signed {
					found = found || s == header
				}
				if !found {
					t.Fatalf("%s is not among the signed headers %v", header, signed)
				}
			}
		})
	}
}

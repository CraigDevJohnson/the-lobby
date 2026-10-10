package soccer

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// SigV4 signs requests to the backend's Lambda address with the site's own
// role, which is the only caller AWS lets through (ADR 0006).
func SigV4(creds aws.CredentialsProvider, region string) func(*http.Request, []byte) error {
	signer := v4.NewSigner()
	return func(r *http.Request, body []byte) error {
		c, err := creds.Retrieve(r.Context())
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		return signer.SignHTTP(r.Context(), c, r, hex.EncodeToString(sum[:]), "lambda", region, time.Now())
	}
}

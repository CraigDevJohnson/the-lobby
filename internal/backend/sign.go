// Package backend holds what the site needs to call any Tool backend.
package backend

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
)

// SigV4 signs requests to a Tool backend's Lambda address with the site's own
// role, which is the only caller AWS lets through (ADR 0006).
func SigV4(creds aws.CredentialsProvider, region string) func(*http.Request, []byte) error {
	signer := v4.NewSigner()
	return func(r *http.Request, body []byte) error {
		c, err := creds.Retrieve(r.Context())
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		hash := hex.EncodeToString(sum[:])
		// A Function URL with IAM auth checks the body against this header
		// and refuses a signed request that lacks it. The SDK's signer only
		// folds the hash into the signature, so the header is set here,
		// before signing, which signs it too.
		r.Header.Set("X-Amz-Content-Sha256", hash)
		return signer.SignHTTP(r.Context(), c, r, hash, "lambda", region, time.Now())
	}
}

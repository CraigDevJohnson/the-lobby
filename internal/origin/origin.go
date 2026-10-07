// Package origin stops requests that reach the server without passing through
// Cloudflare. Cloudflare adds a secret header on every request to the site
// (see ADR 0005); anyone calling the API Gateway address directly lacks it.
package origin

import (
	"crypto/subtle"
	"net/http"
)

const Header = "X-Origin-Secret"

// Require returns middleware that rejects requests whose secret header does
// not match. An empty secret disables the check, which is only acceptable on
// a developer's computer.
func Require(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if secret == "" {
			return next
		}
		want := []byte(secret)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got := []byte(r.Header.Get(Header))
			if subtle.ConstantTimeCompare(got, want) != 1 {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			r.Header.Del(Header)
			next.ServeHTTP(w, r)
		})
	}
}

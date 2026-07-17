// Package middleware provides HTTP middleware for the REST API.
package middleware

import (
	"net/http"
	"strings"

	"github.com/zhukovsd/roadmap-projects-api-test-runner/internal/config"
)

func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token != config.Get().RestAPIKey {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

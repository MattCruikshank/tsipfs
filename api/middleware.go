package api

import (
	"log"
	"net/http"

	"tailscale.com/tsnet"
)

// TailscaleAuth is middleware that verifies the caller's Tailscale identity.
// It sets X-Tailscale-User header with the authenticated user's login name.
func TailscaleAuth(srv *tsnet.Server, next http.Handler) http.Handler {
	lc, err := srv.LocalClient()
	if err != nil {
		log.Fatalf("failed to get tsnet LocalClient: %v", err)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		who, err := lc.WhoIs(r.Context(), r.RemoteAddr)
		if err != nil {
			log.Printf("auth: WhoIs failed for %s: %v", r.RemoteAddr, err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		login := who.UserProfile.LoginName
		r.Header.Set("X-Tailscale-User", login)
		log.Printf("auth: %s %s from %s", r.Method, r.URL.Path, login)
		next.ServeHTTP(w, r)
	})
}

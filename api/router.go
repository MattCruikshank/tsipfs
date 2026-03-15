package api

import (
	"net/http"
	"time"

	"github.com/MattCruikshank/tsipfs/node"
)

// NewRouter creates the REST API router with all endpoints.
func NewRouter(n *node.Node, startTime time.Time) http.Handler {
	mux := http.NewServeMux()

	ph := &PinHandler{Node: n}
	mux.HandleFunc("POST /api/v1/pins", ph.Upload)
	mux.HandleFunc("GET /api/v1/pins", ph.List)
	mux.HandleFunc("GET /api/v1/pins/{cid}", ph.Get)
	mux.HandleFunc("DELETE /api/v1/pins/{cid}", ph.Delete)

	ch := &CacheHandler{Node: n}
	mux.HandleFunc("POST /api/v1/cache/flush", ch.Flush)
	mux.HandleFunc("GET /api/v1/cache/status", ch.Status)

	sh := &StatusHandler{Node: n, StartTime: startTime}
	mux.HandleFunc("GET /api/v1/status", sh.NodeStatus)
	mux.HandleFunc("GET /api/v1/peers", sh.Peers)

	return mux
}

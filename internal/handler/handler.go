package handler

import "net/http"

// New returns the HTTP handler with all routes registered.
func New() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /price", price)
	return mux
}

func price(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

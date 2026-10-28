package handler

import (
	"net/http"
	"snsgo/internal/supplier"
)

type Handler struct {
	suppliers []supplier.Supplier
}

// New returns the HTTP handler with all routes registered.
func New(suppliers []supplier.Supplier) http.Handler {
	h := Handler{suppliers: suppliers}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /price", h.price)

	return mux
}

func (h *Handler) price(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

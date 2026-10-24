package main

import (
	"encoding/json"
	"log"
	"net/http"
	"context"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := map[string]string{
		"status":  "ok",
		"service": "order-service",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("failed to encode health response: %v", err)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func main() {
    ctx := context.Background()

    pool, err := newDBPool(ctx)
    if err != nil {
        log.Fatal(err)
    }
    defer pool.Close()

    mux := http.NewServeMux()
    mux.HandleFunc("GET /health", healthHandler)

    handler := loggingMiddleware(mux)

    log.Println("Order service listening on port:8080")
    if err := http.ListenAndServe(":8080", handler); err != nil {
        log.Fatal(err)
    }
}

package main

import (
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "time"
)

// startAPI serves the collection's endpoints locally, so the request scene
// always gets a real response and the video never depends on the network.
func startAPI() *httptest.Server {
    mux := http.NewServeMux()
    users := []map[string]any{
        {"id": 1, "name": "Ada Lovelace", "email": "ada@example.com", "role": "admin"},
        {"id": 2, "name": "Grace Hopper", "email": "grace@example.com", "role": "engineer"},
        {"id": 3, "name": "Alan Turing", "email": "alan@example.com", "role": "engineer"},
        {"id": 4, "name": "Katherine Johnson", "email": "katherine@example.com", "role": "analyst"},
        {"id": 5, "name": "Margaret Hamilton", "email": "margaret@example.com", "role": "lead"},
    }
    mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
        time.Sleep(180 * time.Millisecond) // a visible round trip
        w.Header().Set("Content-Type", "application/json; charset=utf-8")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("X-Request-Id", "9f2c41ab-7e05-4d3a")
        if r.Method == http.MethodPost {
            w.WriteHeader(http.StatusCreated)
            _ = json.NewEncoder(w).Encode(map[string]any{
                "id": 6, "name": "Ada Lovelace", "email": "ada@example.com", "created": true,
            })
            return
        }
        _ = json.NewEncoder(w).Encode(map[string]any{"page": 1, "total": 5, "users": users})
    })
    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "path": r.URL.Path})
    })
    return httptest.NewServer(mux)
}

package main

import (
	"encoding/json"
	"net/http"
)

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "sprint": "1"})
}

func main() {
	http.HandleFunc("/healthz", healthHandler)
	_ = http.ListenAndServe(":8080", nil)
}

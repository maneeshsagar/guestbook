// guestbook/api stores entries in a SQLite file on the platform's /data
// volume — no managed database, just the disk this app was given.
package main

import (
	"database/sql"
	"encoding/json"
	"html"
	"log"
	"net/http"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func js(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func main() {
	var err error
	db, err = sql.Open("sqlite", "/data/guestbook.db")
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		message TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now'))
	)`); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{"ok": true, "service": "guestbook-api"})
	})

	http.HandleFunc("/api/entries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct{ Name, Message string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				js(w, 400, map[string]string{"error": "invalid body"})
				return
			}
			name := strings.TrimSpace(body.Name)
			message := strings.TrimSpace(body.Message)
			if name == "" || message == "" {
				js(w, 400, map[string]string{"error": "name and message are required"})
				return
			}
			if len(name) > 60 {
				name = name[:60]
			}
			if len(message) > 500 {
				message = message[:500]
			}
			// Stored escaped: this is rendered as raw HTML by the frontend
			// below (a guestbook is exactly the kind of app that would get
			// this wrong), so escape once here rather than trust every
			// future renderer to do it.
			if _, err := db.Exec(`INSERT INTO entries (name, message) VALUES (?, ?)`, html.EscapeString(name), html.EscapeString(message)); err != nil {
				js(w, 500, map[string]string{"error": err.Error()})
				return
			}
			fallthrough
		case http.MethodGet:
			rows, err := db.Query(`SELECT name, message, created_at FROM entries ORDER BY id DESC LIMIT 200`)
			if err != nil {
				js(w, 500, map[string]string{"error": err.Error()})
				return
			}
			defer rows.Close()
			type entry struct{ Name, Message, CreatedAt string }
			out := []entry{}
			for rows.Next() {
				var e entry
				rows.Scan(&e.Name, &e.Message, &e.CreatedAt)
				out = append(out, e)
			}
			js(w, 200, map[string]any{"entries": out})
		default:
			js(w, 405, map[string]string{"error": "method not allowed"})
		}
	})

	http.HandleFunc("/api/whoami", func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{"gate_email": r.Header.Get("X-Gate-Email"), "gate_app": r.Header.Get("X-Gate-App")})
	})

	log.Println("guestbook-api listening on 8080, db at /data/guestbook.db")
	log.Fatal(http.ListenAndServe("0.0.0.0:8080", nil))
}

// guestbook/api stores entries in a SQLite file on the platform's /data
// volume — no managed database, just the disk this app was given.
package main

import (
	"database/sql"
	"encoding/json"
	"html"
	"log"
	"net/http"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func js(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

// hasColumn checks sqlite's own table metadata rather than trying an ALTER
// and swallowing the "duplicate column" error — that would also swallow a
// genuinely different failure (disk full, corrupt file) and hide it as if
// the migration had already run.
func hasColumn(table, col string) bool {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		log.Fatalf("migrate: inspect %s: %v", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		rows.Scan(&name)
		if name == col {
			return true
		}
	}
	return false
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

	// v2 schema migration, applied in place so existing rows in a
	// redeployed app's /data volume survive:
	//   - add  : optional "email" column
	//   - rename: "message" -> "body" (same data, new name)
	//   - drop+recreate: "created_at" replaced by "updated_at"
	// Each step is guarded by hasColumn so a redeploy of an already-migrated
	// database is a no-op rather than a duplicate-column error.
	if !hasColumn("entries", "email") {
		if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN email TEXT`); err != nil {
			log.Fatalf("migrate: add email: %v", err)
		}
	}
	if hasColumn("entries", "message") && !hasColumn("entries", "body") {
		if _, err := db.Exec(`ALTER TABLE entries RENAME COLUMN message TO body`); err != nil {
			log.Fatalf("migrate: rename message->body: %v", err)
		}
	}
	if hasColumn("entries", "created_at") {
		if _, err := db.Exec(`ALTER TABLE entries ADD COLUMN updated_at TEXT`); err != nil {
			log.Fatalf("migrate: add updated_at: %v", err)
		}
		if _, err := db.Exec(`UPDATE entries SET updated_at = created_at WHERE updated_at IS NULL`); err != nil {
			log.Fatalf("migrate: backfill updated_at: %v", err)
		}
		if _, err := db.Exec(`ALTER TABLE entries DROP COLUMN created_at`); err != nil {
			log.Fatalf("migrate: drop created_at: %v", err)
		}
	}

	// New compose-declared env var (WELCOME_MESSAGE): surfaced through
	// /api/health so the redeploy test can confirm it reached the running
	// container without a code change to how health itself works.
	welcome := os.Getenv("WELCOME_MESSAGE")

	http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		js(w, 200, map[string]any{"ok": true, "service": "guestbook-api", "welcome": welcome})
	})

	http.HandleFunc("/api/entries", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var body struct{ Name, Message, Email string }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				js(w, 400, map[string]string{"error": "invalid body"})
				return
			}
			name := strings.TrimSpace(body.Name)
			message := strings.TrimSpace(body.Message)
			email := strings.TrimSpace(body.Email)
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
			if _, err := db.Exec(`INSERT INTO entries (name, body, email, updated_at) VALUES (?, ?, ?, datetime('now'))`, html.EscapeString(name), html.EscapeString(message), html.EscapeString(email)); err != nil {
				js(w, 500, map[string]string{"error": err.Error()})
				return
			}
			fallthrough
		case http.MethodGet:
			rows, err := db.Query(`SELECT name, body, email, updated_at FROM entries ORDER BY id DESC LIMIT 200`)
			if err != nil {
				js(w, 500, map[string]string{"error": err.Error()})
				return
			}
			defer rows.Close()
			type entry struct {
				Name, Body, UpdatedAt string
				Email                 sql.NullString
			}
			out := []map[string]any{}
			for rows.Next() {
				var e entry
				rows.Scan(&e.Name, &e.Body, &e.Email, &e.UpdatedAt)
				out = append(out, map[string]any{
					"Name": e.Name, "Body": e.Body, "Email": e.Email.String, "UpdatedAt": e.UpdatedAt,
				})
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

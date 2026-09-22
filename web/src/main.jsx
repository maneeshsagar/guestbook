import { createRoot } from "react-dom/client";
import { useEffect, useState } from "react";

function App() {
  const [name, setName] = useState("");
  const [message, setMessage] = useState("");
  const [entries, setEntries] = useState([]);
  const [error, setError] = useState("");

  async function load() {
    const r = await fetch("/api/entries");
    const j = await r.json();
    setEntries(j.entries || []);
  }
  useEffect(() => { load(); }, []);

  async function submit(e) {
    e.preventDefault();
    setError("");
    const r = await fetch("/api/entries", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ name, message }),
    });
    const j = await r.json();
    if (!r.ok) { setError(j.error || "failed"); return; }
    setName(""); setMessage("");
    setEntries(j.entries || []);
  }

  return (
    <div style={{ fontFamily: "system-ui, sans-serif", maxWidth: 560, margin: "3rem auto", padding: "0 1rem" }}>
      <h1>Guestbook</h1>
      <p style={{ color: "#666" }}>Sign in, leave a note. Stored in a SQLite file on this app's own disk.</p>
      <form onSubmit={submit} style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Your name" required />
        <textarea value={message} onChange={(e) => setMessage(e.target.value)} placeholder="Leave a message" rows={3} required />
        <button type="submit">Sign the guestbook</button>
      </form>
      {error && <p style={{ color: "crimson" }}>{error}</p>}
      <ul style={{ marginTop: "2rem", paddingLeft: 0, listStyle: "none" }}>
        {entries.map((e, i) => (
          <li key={i} style={{ padding: "10px 0", borderBottom: "1px solid #eee" }}>
            <strong dangerouslySetInnerHTML={{ __html: e.Name }} />
            <span style={{ color: "#999" }}> — {e.CreatedAt}</span>
            <p dangerouslySetInnerHTML={{ __html: e.Message }} />
          </li>
        ))}
        {entries.length === 0 && <li style={{ color: "#999" }}>No entries yet — be the first.</li>}
      </ul>
    </div>
  );
}

createRoot(document.getElementById("root")).render(<App />);

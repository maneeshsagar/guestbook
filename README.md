# guestbook

Visitors leave a name and a short message; the wall lists everyone who has
signed. No managed database — the backend keeps a SQLite file on the
platform's own /data volume, so the point of this fixture is proving that
file survives a restart/redeploy (unlike Postgres, nothing provisions or
protects this for you).

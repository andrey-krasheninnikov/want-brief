"""Exercise the in-service workspace preparation on a disposable single-owner v48 copy.

DATABASE_URL must point to wb_legacy_prepare. Pass a v48 binary and its migrations,
then the new binary. Run from the new source checkout with Python 3 and psql.
"""

import json
import os
import socket
import subprocess
import sys
import time
from contextlib import contextmanager
from pathlib import Path
from urllib.error import URLError
from urllib.request import Request, urlopen
from uuid import UUID


def sql(query):
    result = subprocess.run(
        ["psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1", "-d", os.environ["DATABASE_URL"], "-c", query],
        text=True, stdout=subprocess.PIPE,
    )
    if result.returncode:
        raise RuntimeError("PostgreSQL check failed")
    return result.stdout.strip()


def version():
    return sql("SELECT max(version_id) FROM goose_db_version WHERE is_applied")


def http(root, method, path, cookie=None, data=None):
    headers = {"Content-Type": "application/json"}
    if cookie:
        headers["Cookie"] = cookie
    body = json.dumps(data).encode() if data is not None else None
    with urlopen(Request(root + path, method=method, data=body, headers=headers), timeout=5) as res:
        raw = res.read()
        return (json.loads(raw) if raw else None, res.headers)


@contextmanager
def server(binary, migrations):
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        port = sock.getsockname()[1]
    host = f"127.0.0.1:{port}"
    env = {**os.environ, "HTTP_ADDR": host, "MIGRATIONS_DIR": str(migrations)}
    with subprocess.Popen([binary], env=env, stdout=subprocess.DEVNULL) as child:
        try:
            for _ in range(100):
                if child.poll() is not None:
                    raise RuntimeError("server exited before becoming ready")
                try:
                    http("http://" + host, "GET", "/api/health")
                    break
                except URLError:
                    time.sleep(0.05)
            else:
                raise RuntimeError("server did not become ready")
            yield "http://" + host
        finally:
            child.terminate()


def run(old, old_migrations, new):
    assert sql("SELECT current_database()") == "wb_legacy_prepare", "use a disposable wb_legacy_prepare copy"
    assert version() == "48"
    assert sql("SELECT count(*) FROM users") == "1"
    assert sql("SELECT count(*) FROM sources WHERE token_sealed<>'' AND sync_interval_minutes>0") == "0"
    owner = UUID(sql("SELECT id FROM users"))
    journal_rows = sql("SELECT count(*) FROM goose_db_version")
    password = os.environ.get("LEGACY_TEST_PASSWORD", "sample-password")
    new_env = {**os.environ, "MIGRATIONS_DIR": str(Path("migrations").resolve())}

    with server(old, old_migrations) as root:
        _, headers = http(root, "POST", "/api/login", data={"password": password})
        cookie = headers["Set-Cookie"].split(";", 1)[0]
        note, _ = http(root, "POST", "/api/notes", cookie, {"body": "Before prepare"})
        before = UUID(note["id"])
        subprocess.run([new, "prepare-workspaces"], env=new_env, check=True)
        assert version() == "48"
        assert sql("SELECT count(*) FROM goose_db_version") == journal_rows
        assert sql(f"SELECT workspace_id=owner_id AND owner_id='{owner}' FROM notes WHERE id='{before}'") == "t"
        assert sql("""SELECT is_nullable FROM information_schema.columns WHERE table_schema='public'
                      AND table_name='notes' AND column_name='owner_id'""") == "YES"
        note, _ = http(root, "POST", "/api/notes", cookie, {"body": "After prepare"})
        after = UUID(note["id"])
        assert sql(f"SELECT workspace_id=owner_id AND owner_id='{owner}' FROM notes WHERE id='{after}'") == "t"
        record, _ = http(root, "POST", "/api/projects", cookie, {"name": "Before cutover"})
        project = UUID(record["id"])
        http(root, "GET", f"/api/projects/{project}", cookie)
        http(root, "PATCH", f"/api/projects/{project}", cookie, {"archived": True})
        http(root, "DELETE", f"/api/projects/{project}", cookie)
        for amount in (1, 2):
            day, _ = http(root, "PUT", "/api/earn/days/2090-01-02", cookie, {"usd": amount, "rate": 1})
            assert day["usd"] == amount

    with server(old, old_migrations) as root:
        notes, _ = http(root, "GET", "/api/notes", cookie)
        assert {str(before), str(after)} <= {note["id"] for note in notes}
        subprocess.run([new, "prepare-workspaces"], env=new_env, check=True)
        assert version() == "48"
        assert sql("SELECT count(*) FROM goose_db_version") == journal_rows
    subprocess.run([new, "-migrate-only"], env=new_env, check=True)
    assert version() == "52"
    assert sql(f"SELECT id=owner_id AND legacy_paid FROM workspaces WHERE owner_id='{owner}'") == "t"
    assert sql(f"""SELECT count(*) FROM notes WHERE workspace_id='{owner}' AND owner_id='{owner}'
                   AND id IN ('{before}', '{after}')""") == "2"
    assert sql(f"SELECT usd=2 FROM earn_days WHERE workspace_id='{owner}' AND owner_id='{owner}' AND day='2090-01-02'") == "t"
    with server(new, new_env["MIGRATIONS_DIR"]) as root:
        account, _ = http(root, "GET", "/api/me", cookie)
        assert account["user"]["id"] == account["workspace"]["id"] == str(owner)
        assert account["workspace"]["plan"] == "paid"
        notes, _ = http(root, "GET", "/api/notes", cookie)
        assert {str(before), str(after)} <= {note["id"] for note in notes}
    print("v48 restart, CRUD, upsert, repeated preparation and v52 cutover passed")


if __name__ == "__main__":
    if not __debug__:
        sys.exit("run without -O to enable the database checks")
    if len(sys.argv) != 4:
        sys.exit("usage: python3 tests/legacy_prepare.py <v48-binary> <v48-migrations> <new-binary>")
    run(*(str(Path(value).resolve()) for value in sys.argv[1:]))

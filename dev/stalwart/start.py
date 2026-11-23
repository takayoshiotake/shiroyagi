"""Configure and seed the isolated Shiroyagi development mail server."""

import base64
from contextlib import contextmanager
from email.parser import BytesHeaderParser
import imaplib
import json
import os
from pathlib import Path
import secrets
import signal
import socket
import subprocess
import time
from urllib.request import ProxyHandler, Request, build_opener

DATA = Path(os.environ.get("STALWART_DEV_DATA_DIR", "/var/lib/stalwart"))
CONFIG = DATA / "config.json"
CONFIGURED = DATA / ".shiroyagi-configured"
READY = Path("/tmp/shiroyagi-stalwart-ready")
DOMAIN = "example.test"
MAIL_PASSWORD = "shiroyagi-dev"
RECOVERY_PORT = 18080
IMAP_PORT = 31143
SMTP_PORT = 1025


@contextmanager
def run_server(**settings):
    process = subprocess.Popen(
        ["stalwart", "--config", str(CONFIG)], env={**os.environ, **settings}
    )
    try:
        yield process
    finally:
        if process.poll() is None:
            process.terminate()
            try:
                process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()


def wait_for_port(process, port):
    deadline = time.monotonic() + 60
    while process.poll() is None and time.monotonic() < deadline:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=1):
                return
        except OSError:
            time.sleep(0.25)
    raise RuntimeError(f"Stalwart did not open port {port} (exit: {process.poll()})")


def configure(password):
    authorization = base64.b64encode(f"recovery:{password}".encode()).decode()
    http = build_opener(ProxyHandler({}))

    def call(method, arguments):
        method = "x:" + method
        request = Request(
            f"http://127.0.0.1:{RECOVERY_PORT}/jmap",
            data=json.dumps({
                "using": ["urn:ietf:params:jmap:core", "urn:stalwart:jmap"],
                "methodCalls": [[method, arguments, "dev"]],
            }).encode(),
            headers={
                "Authorization": f"Basic {authorization}",
                "Content-Type": "application/json",
            },
        )
        with http.open(request, timeout=10) as response:
            name, result, _ = json.load(response)["methodResponses"][0]
        if name != method:
            raise RuntimeError(f"Stalwart {method} failed: {result}")
        for key in ("notCreated", "notUpdated"):
            if errors := result.get(key):
                raise RuntimeError(f"Stalwart {method} failed: {errors}")
        return result

    def ensure(object_type, objects):
        existing = {
            entry["name"]: entry["id"]
            for entry in call(f"{object_type}/get", {"properties": ["id", "name"]})["list"]
        }
        create = {name: value for name, value in objects.items() if name not in existing}
        if create:
            result = call(f"{object_type}/set", {"create": create})
            existing.update({name: item["id"] for name, item in result["created"].items()})
        return existing

    def update(object_type, value):
        call(f"{object_type}/set", {"update": {"singleton": value}})

    domain_id = ensure("Domain", {
        DOMAIN: {
            "name": DOMAIN,
            "certificateManagement": {"@type": "Manual"},
            "dkimManagement": {"@type": "Manual"},
            "dnsManagement": {"@type": "Manual"},
            "allowRelaying": False,
        },
    })[DOMAIN]
    update("SystemSettings", {
        "defaultHostname": f"mail.{DOMAIN}", "defaultDomainId": domain_id,
    })
    ensure("Account", {
        name: {
            "@type": "User",
            "name": name,
            "domainId": domain_id,
            "credentials": {"0": {"@type": "Password", "secret": MAIL_PASSWORD}},
        }
        for name in ("dev", "alice", "bob", "carol", "notifications", "ops")
    })
    ensure("NetworkListener", {
        protocol.lower(): {
            "name": protocol.lower(),
            "protocol": protocol.lower(),
            "bind": {f"0.0.0.0:{port}": True},
            "useTls": False,
        }
        for protocol, port in (("Imap", IMAP_PORT), ("Smtp", SMTP_PORT))
    })
    update("Imap", {"allowPlainTextAuth": True})
    update("MtaStageAuth", {"saslMechanisms": {"else": "plain"}})
    update("MtaStageRcpt", {"allowRelaying": {"else": "false"}})
    update("MtaOutboundStrategy", {"route": {"else": "'local'"}})


def seed_inbox():
    fixtures = Path(__file__).with_name("fixtures")
    parser = BytesHeaderParser()
    with imaplib.IMAP4("127.0.0.1", IMAP_PORT, timeout=10) as client:
        client.login(f"dev@{DOMAIN}", MAIL_PASSWORD)
        status, count = client.select("INBOX")
        if status != "OK":
            raise RuntimeError("Could not open the development INBOX")
        message_ids = set()
        if int(count[0]):
            # Read stored headers directly instead of relying on the search index.
            status, headers = client.fetch("1:*", "(BODY.PEEK[HEADER.FIELDS (MESSAGE-ID)])")
            if status != "OK":
                raise RuntimeError("Could not read Message-IDs from the development INBOX")
            message_ids.update(
                parser.parsebytes(header[1])["Message-ID"]
                for header in headers if isinstance(header, tuple)
            )
        for path in sorted(fixtures.glob("*.eml")):
            message = path.read_bytes()
            message_id = parser.parsebytes(message)["Message-ID"]
            if not message_id:
                raise RuntimeError(f"Missing Message-ID in {path.name}")
            if message_id in message_ids:
                continue
            if client.append("INBOX", None, None, message)[0] != "OK":
                raise RuntimeError(f"Could not append {path.name} to the development INBOX")
            message_ids.add(message_id)


def main():
    READY.unlink(missing_ok=True)
    if not CONFIGURED.exists():
        CONFIG.write_text(json.dumps({"@type": "RocksDb", "path": str(DATA / "db")}))
        password = secrets.token_urlsafe(32)
        with run_server(
            STALWART_RECOVERY_MODE="1",
            STALWART_RECOVERY_MODE_PORT=str(RECOVERY_PORT),
            STALWART_RECOVERY_ADMIN=f"recovery:{password}",
        ) as process:
            wait_for_port(process, RECOVERY_PORT)
            configure(password)
        CONFIGURED.touch()

    with run_server() as process:
        try:
            wait_for_port(process, IMAP_PORT)
            wait_for_port(process, SMTP_PORT)
            seed_inbox()
            READY.touch()
            return process.wait()
        finally:
            READY.unlink(missing_ok=True)


def stop(_signum, _frame):
    raise SystemExit(0)


if __name__ == "__main__":
    signal.signal(signal.SIGTERM, stop)
    raise SystemExit(main())

#!/usr/bin/env python3
"""Zero-dependency static file server for the Page Pulse frontend.

Serves web/index.html, style.css, and app.js, and exposes a /config.js
endpoint that injects BACKEND_URL as window.API_BASE_URL, since a plain
static page has no other way to read an environment variable at runtime.
"""

import http.server
import os
import socketserver

WEB_DIR = os.path.dirname(os.path.abspath(__file__))


def load_env_file(path):
    if not os.path.exists(path):
        return

    with open(path) as f:
        for line in f:
            line = line.strip()
            if not line or line.startswith("#") or "=" not in line:
                continue

            key, value = line.split("=", 1)
            os.environ.setdefault(key.strip(), value.strip())


load_env_file(os.path.join(WEB_DIR, ".local.env"))

PORT = int(os.environ.get("PORT", "3000"))
BACKEND_URL = os.environ.get("BACKEND_URL", "http://localhost:8000")


class Handler(http.server.SimpleHTTPRequestHandler):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, directory=WEB_DIR, **kwargs)

    def do_GET(self):
        if self.path == "/config.js":
            body = f'window.API_BASE_URL = "{BACKEND_URL}";'.encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/javascript")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return

        super().do_GET()

    def log_message(self, fmt, *args):
        print("[web]", fmt % args)


if __name__ == "__main__":
    with socketserver.TCPServer(("", PORT), Handler) as httpd:
        print(f"Page Pulse frontend running at http://localhost:{PORT}")
        print(f"Talking to backend at {BACKEND_URL}")
        httpd.serve_forever()

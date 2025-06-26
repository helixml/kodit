"""Knock knock server for testing."""

import json
import secrets
from http.server import BaseHTTPRequestHandler, HTTPServer

from rich.console import Console

# Store knock counts and tokens
knock_counts: dict[str, int] = {}
console = Console()


class KnockAuthHandler(BaseHTTPRequestHandler):
    """Knock knock server handler."""

    def _set_headers(self, status_code: int = 200) -> None:
        self.send_response(status_code)
        self.send_header("Content-type", "application/json")
        self.end_headers()

    def _get_token_from_header(self) -> str:
        auth_header = self.headers.get("Authorization", "")
        if auth_header.startswith("Bearer "):
            return auth_header.split(" ")[1]
        return ""

    def do_POST(self) -> None:  # noqa: N802
        """Handle POST requests."""
        if self.path == "/knock":
            token = self._get_token_from_header()
            if not token:
                token = secrets.token_urlsafe(16)
                knock_counts[token] = 0

            knock_counts[token] += 1
            console.print(
                f"[green]Knock {knock_counts[token]} received for token {token}[/green]"
            )

            response = {
                "message": f"Knock {knock_counts[token]} received",
                "token": token,
                "knocks_remaining": max(0, 3 - knock_counts[token]),
            }

            self._set_headers()
            self.wfile.write(json.dumps(response).encode())
        else:
            self._set_headers(404)
            self.wfile.write(json.dumps({"error": "Not found"}).encode())

    def do_GET(self) -> None:  # noqa: N802
        """Handle GET requests."""
        if self.path == "/secret":
            token = self._get_token_from_header()
            if not token or knock_counts.get(token, 0) < 3:
                self._set_headers(401)
                self.wfile.write(
                    json.dumps(
                        {
                            "error": "Unauthorized",
                            "message": "You need to knock three times first!",
                        }
                    ).encode()
                )
                return

            self._set_headers()
            self.wfile.write(
                json.dumps(
                    {
                        "message": "Welcome to the secret area!",
                        "secret": "The answer is 42",
                    }
                ).encode()
            )
        else:
            self._set_headers(404)
            self.wfile.write(json.dumps({"error": "Not found"}).encode())


def run_server(port: int = 8000) -> None:
    """Run the knock knock server."""
    server_address = ("", port)
    httpd = HTTPServer(server_address, KnockAuthHandler)
    console.print(f"[bold blue]Starting server on port {port}[/bold blue]")
    console.print("[yellow]To access the secret area:[/yellow]")
    console.print("1. Make 3 POST requests to /knock")
    console.print("2. Use the returned token in the Authorization header")
    console.print("3. Make a GET request to /secret with the token")
    httpd.serve_forever()


if __name__ == "__main__":
    run_server()

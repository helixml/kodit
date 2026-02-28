#!/usr/bin/env python3
"""CLI wrapper for Kodit MCP endpoint.

Speaks JSON-RPC to the Kodit MCP server over HTTP. Uses only stdlib so it can
run inside SWE-bench Docker containers with zero extra dependencies.

Usage:
    python /kodit-cli.py semantic-search --query "find auth handler"
    python /kodit-cli.py keyword-search --keywords "auth login handler"
    python /kodit-cli.py read-file --uri "file://1/abc123/path/to/file.py"
    python /kodit-cli.py list-repositories
    python /kodit-cli.py architecture --repo-url "github.com/foo/bar"
    python /kodit-cli.py api-docs --repo-url "github.com/foo/bar"
    python /kodit-cli.py commit-description --repo-url "github.com/foo/bar"
    python /kodit-cli.py database-schema --repo-url "github.com/foo/bar"
    python /kodit-cli.py cookbook --repo-url "github.com/foo/bar"
    python /kodit-cli.py version
"""

import argparse
import json
import os
import sys
import urllib.error
import urllib.request

SESSION_FILE = "/tmp/.kodit_session"  # noqa: S108


def _mcp_url():
    """Read the MCP endpoint URL from the environment."""
    url = os.environ.get("KODIT_MCP_URL", "")
    if not url:
        print("ERROR: KODIT_MCP_URL environment variable is not set", file=sys.stderr)
        sys.exit(1)
    return url


def _load_session():
    """Load cached session ID from disk."""
    try:
        with open(SESSION_FILE) as f:
            return f.read().strip()
    except FileNotFoundError:
        return ""


def _save_session(session_id):
    """Persist session ID to disk for subsequent invocations."""
    with open(SESSION_FILE, "w") as f:
        f.write(session_id)


def _post(url, payload, session_id=""):
    """Send a JSON-RPC request and return the parsed response + headers."""
    body = json.dumps(payload).encode()
    req = urllib.request.Request(  # noqa: S310
        url,
        data=body,
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    if session_id:
        req.add_header("Mcp-Session-Id", session_id)

    try:
        resp = urllib.request.urlopen(req, timeout=120)  # noqa: S310
    except urllib.error.HTTPError as exc:
        error_body = exc.read().decode(errors="replace")
        print(f"ERROR: HTTP {exc.code}: {error_body}", file=sys.stderr)
        sys.exit(1)
    except urllib.error.URLError as exc:
        print(f"ERROR: Cannot reach Kodit server: {exc.reason}", file=sys.stderr)
        sys.exit(1)

    resp_session = resp.headers.get("Mcp-Session-Id", "")
    data = json.loads(resp.read().decode())
    return data, resp_session


def _ensure_session(url):
    """Return a valid session ID, initialising if necessary."""
    session_id = _load_session()
    if session_id:
        return session_id

    payload = {
        "jsonrpc": "2.0",
        "id": 0,
        "method": "initialize",
        "params": {
            "protocolVersion": "2025-06-18",
            "capabilities": {},
            "clientInfo": {"name": "kodit-cli", "version": "1.0.0"},
        },
    }
    data, resp_session = _post(url, payload)

    if "error" in data:
        print(f"ERROR: MCP initialize failed: {data['error']}", file=sys.stderr)
        sys.exit(1)

    session_id = resp_session
    if session_id:
        _save_session(session_id)
    return session_id


def _call_tool(tool_name, arguments):
    """Call an MCP tool and print the result as plain text."""
    url = _mcp_url()
    session_id = _ensure_session(url)

    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "tools/call",
        "params": {
            "name": tool_name,
            "arguments": arguments,
        },
    }
    data, resp_session = _post(url, payload, session_id)

    # Update session if the server sent a new one
    if resp_session and resp_session != session_id:
        _save_session(resp_session)

    if "error" in data:
        code = data["error"].get("code", "")
        message = data["error"].get("message", "")
        print(f"ERROR: JSON-RPC error {code}: {message}", file=sys.stderr)
        sys.exit(1)

    result = data.get("result", {})
    contents = result.get("content", [])
    is_error = result.get("isError", False)

    text_parts = [c.get("text", "") for c in contents if c.get("type") == "text"]
    output = "\n".join(text_parts)

    if is_error:
        print(f"Tool error: {output}", file=sys.stderr)
        sys.exit(1)

    print(output)


def _read_resource(uri):
    """Read an MCP resource by URI and print its content."""
    url = _mcp_url()
    session_id = _ensure_session(url)
    payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "resources/read",
        "params": {"uri": uri},
    }
    data, resp_session = _post(url, payload, session_id)
    if resp_session and resp_session != session_id:
        _save_session(resp_session)
    if "error" in data:
        code = data["error"].get("code", "")
        message = data["error"].get("message", "")
        print(f"ERROR: JSON-RPC error {code}: {message}", file=sys.stderr)
        sys.exit(1)
    result = data.get("result", {})
    contents = result.get("contents", [])
    for c in contents:
        print(c.get("text", ""))


def cmd_read_file(args):
    """Read a file using its Kodit resource URI."""
    _read_resource(args.uri)


def cmd_semantic_search(args):
    """Execute the semantic_search tool."""
    arguments = {"query": args.query}
    if args.language:
        arguments["language"] = args.language
    if args.source_repo:
        arguments["source_repo"] = args.source_repo
    if args.limit:
        arguments["limit"] = args.limit
    _call_tool("semantic_search", arguments)


def cmd_keyword_search(args):
    """Execute the keyword_search tool."""
    arguments = {"keywords": args.keywords}
    if args.language:
        arguments["language"] = args.language
    if args.source_repo:
        arguments["source_repo"] = args.source_repo
    if args.limit:
        arguments["limit"] = args.limit
    _call_tool("keyword_search", arguments)


def cmd_version(_args):
    """Execute the get_version tool."""
    _call_tool("get_version", {})


def cmd_list_repositories(_args):
    """Execute the list_repositories tool."""
    _call_tool("list_repositories", {})


def cmd_architecture(args):
    """Execute the get_architecture_docs tool."""
    arguments = {"repo_url": args.repo_url}
    if args.commit_sha:
        arguments["commit_sha"] = args.commit_sha
    _call_tool("get_architecture_docs", arguments)


def cmd_api_docs(args):
    """Execute the get_api_docs tool."""
    arguments = {"repo_url": args.repo_url}
    if args.commit_sha:
        arguments["commit_sha"] = args.commit_sha
    _call_tool("get_api_docs", arguments)


def cmd_commit_description(args):
    """Execute the get_commit_description tool."""
    arguments = {"repo_url": args.repo_url}
    if args.commit_sha:
        arguments["commit_sha"] = args.commit_sha
    _call_tool("get_commit_description", arguments)


def cmd_database_schema(args):
    """Execute the get_database_schema tool."""
    arguments = {"repo_url": args.repo_url}
    if args.commit_sha:
        arguments["commit_sha"] = args.commit_sha
    _call_tool("get_database_schema", arguments)


def cmd_cookbook(args):
    """Execute the get_cookbook tool."""
    arguments = {"repo_url": args.repo_url}
    if args.commit_sha:
        arguments["commit_sha"] = args.commit_sha
    _call_tool("get_cookbook", arguments)


def _add_repo_args(subparser):
    """Add common --repo-url and --commit-sha arguments."""
    subparser.add_argument("--repo-url", required=True, help="Repository URL")
    subparser.add_argument("--commit-sha", default="", help="Optional commit SHA")


def main():
    """Entry point."""
    parser = argparse.ArgumentParser(
        description="CLI wrapper for Kodit MCP tools",
        prog="kodit-cli",
    )
    subparsers = parser.add_subparsers(dest="command", required=True)

    # semantic-search
    sp_sem = subparsers.add_parser(
        "semantic-search", help="Semantic similarity search"
    )
    sp_sem.add_argument(
        "--query", required=True, help="Natural language description of what you are looking for"
    )
    sp_sem.add_argument("--language", default="", help="Filter by file extension (e.g. .go, .py)")
    sp_sem.add_argument("--source-repo", default="", help="Filter by source repository URL")
    sp_sem.add_argument("--limit", type=int, default=0, help="Maximum number of results (default 10)")
    sp_sem.set_defaults(func=cmd_semantic_search)

    # read-file
    sp_read = subparsers.add_parser("read-file", help="Read file content by Kodit resource URI")
    sp_read.add_argument("--uri", required=True, help="File resource URI from search results")
    sp_read.set_defaults(func=cmd_read_file)

    # keyword-search
    sp_kw = subparsers.add_parser(
        "keyword-search", help="BM25 keyword-based search"
    )
    sp_kw.add_argument("--keywords", required=True, help="Keywords to search for")
    sp_kw.add_argument("--language", default="", help="Filter by programming language")
    sp_kw.add_argument("--source-repo", default="", help="Filter by source repository URL")
    sp_kw.add_argument("--limit", type=int, default=0, help="Maximum number of results (default 10)")
    sp_kw.set_defaults(func=cmd_keyword_search)

    # list-repositories
    sp_list = subparsers.add_parser(
        "list-repositories", help="List indexed repositories"
    )
    sp_list.set_defaults(func=cmd_list_repositories)

    # architecture
    sp_arch = subparsers.add_parser(
        "architecture", help="Get architecture documentation"
    )
    _add_repo_args(sp_arch)
    sp_arch.set_defaults(func=cmd_architecture)

    # api-docs
    sp_api = subparsers.add_parser("api-docs", help="Get API documentation")
    _add_repo_args(sp_api)
    sp_api.set_defaults(func=cmd_api_docs)

    # commit-description
    sp_commit = subparsers.add_parser(
        "commit-description", help="Get commit description"
    )
    _add_repo_args(sp_commit)
    sp_commit.set_defaults(func=cmd_commit_description)

    # database-schema
    sp_db = subparsers.add_parser("database-schema", help="Get database schema")
    _add_repo_args(sp_db)
    sp_db.set_defaults(func=cmd_database_schema)

    # cookbook
    sp_cook = subparsers.add_parser("cookbook", help="Get cookbook / usage examples")
    _add_repo_args(sp_cook)
    sp_cook.set_defaults(func=cmd_cookbook)

    # version
    sp_ver = subparsers.add_parser("version", help="Get the Kodit server version")
    sp_ver.set_defaults(func=cmd_version)

    args = parser.parse_args()
    args.func(args)


if __name__ == "__main__":
    main()

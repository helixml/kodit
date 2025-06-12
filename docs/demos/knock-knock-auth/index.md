---
title: "Kodit Demo: Proprietary Auth Patterns"
linkTitle: Proprietary Code
description: A demo of Kodit where knowledge of proprietary code is required.
weight: 1
tags:
- demo
- auth
- cursor
---

Another common feature of private codebases is proprietary code to achieve a certain
task. For example, you might a specific way to request data from your data lake, or a
specific format to use when using a common API.

This example uses the analogy of a proprietary authentication mechanism. I have a super
secret server with sensitive data. It is protected with a [custom auth implementation](https://gist.github.com/cbf0bd1f3338ddf9f98879148d2d752d)
that requires the user to "knock three times" before they get access.

## With Cursor

Without Kodit, an AI assistant makes an average assumption about what the auth might
look like, based upon the context provided by the user. The transcript is below. You can
see that Cursor doesn't have a chance.

### Original Cursor Response

> **User**
>
> Write a python script that is able to access my super secret knock knock api on localhost:8000/secret.
>
> **Cursor**
>
> I'll help you create a Python script to interact with your local knock knock API. I'll create a script that makes a > request to the specified endpoint.
>
> Let me write a script that uses the `requests` library to make HTTP requests to your local API:
>
> ```python
> import requests
> 
> def access_secret_api():
>     try:
>         # Make a GET request to the secret endpoint
>         response = requests.get('http://localhost:8000/secret')
>         
>         # Check if the request was successful
>         response.raise_for_status()
>         
>         # Print the response
>         print("Response from secret API:", response.text)
>         
>     except requests.exceptions.RequestException as e:
>         print(f"Error accessing the API: {e}")
> 
> if __name__ == "__main__":
>     access_secret_api()
> ```
>
> I've created a Python script that will:
>
> 1. Make a GET request to `http://localhost:8000/secret`
> 2. Handle any potential errors that might occur during the request
> 3. Print the response from the API if successful
>
> To run this script, you'll need to have the `requests` library installed. You can install it using pip:

## With Kodit

Now let's index the code for my secret server and then leverage Kodit in Cursor.

1. Start afresh. Delete your data directory or your database.
2. Index the [server code](https://gist.github.com/cbf0bd1f3338ddf9f98879148d2d752d): `kodit index https://gist.github.com/cbf0bd1f3338ddf9f98879148d2d752d.git`
3. Launch the MCP server: `kodit serve`kodit
4. Connect Cursor.

The results are shown below. Cursor has correctly queried Kodit, which has responded with examples of the super secret auth implementation. This lead to cursor producing valid, working code that adhered to our esoteric auth implementation!

Here is the result of running the generated code:

```sh
uv run temp.py
Knock 1: Knock 1 received (token: sAQmj2uFrRxc-e9blSCfFg)
Knock 2: Knock 2 received (token: sAQmj2uFrRxc-e9blSCfFg)
Knock 3: Knock 3 received (token: sAQmj2uFrRxc-e9blSCfFg)
Secret area: {'message': 'Welcome to the secret area!', 'secret': 'The answer is 42'}
```

### Kodit Cursor Response

> **User**
>
> Write a python script that is able to access my super secret knock knock api on localhost:8000/secret.
>
> **Cursor**
>
> Based on the information found in the codebase, your "super secret knock knock API" on `localhost:8000/secret` requires a > special authentication flow:
>
> 1. You must make 3 POST requests to `/knock`. Each time, you use the same token (provided in the response) in the > Authorization header as a Bearer token.
> 2. After 3 knocks, you can access `/secret` with a GET request, using the same token in the Authorization header.
>
> Would you like a Python script that automates this entire process (performs the 3 knocks and then accesses the secret), > or do you want just an example of how to access `/secret` assuming you already have a valid token?
>
> **User**
>
> the whole process.
>
> **Cursor**
>
> ```python
> import requests
> 
> BASE_URL = "http://localhost:8000"
> KNOCKS_REQUIRED = 3
> 
> def knock_and_get_secret():
>     token = None
>     headers = {}
>     for i in range(KNOCKS_REQUIRED):
>         if token:
>             headers = {"Authorization": f"Bearer {token}"}
>         else:
>             headers = {}
>         resp = requests.post(f"{BASE_URL}/knock", headers=headers)
>         resp.raise_for_status()
>         data = resp.json()
>         token = data["token"]
>         print(f"Knock {i+1}: {data['message']} (token: {token})")
> 
>     # Now access the secret
>     headers = {"Authorization": f"Bearer {token}"}
>     resp = requests.get(f"{BASE_URL}/secret", headers=headers)
>     if resp.status_code == 200:
>         print("Secret area:", resp.json())
>     else:
>         print("Failed to access secret:", resp.status_code, resp.text)
> 
> if __name__ == "__main__":
>     knock_and_get_secret()
> 
> ```
>
> I've written a Python script in `temp.py` that automates the entire process:
>
> - It performs 3 POST requests to `/knock`, passing the token as required.
> - After the third knock, it makes a GET request to `/secret` with the token.
> - The script prints out the secret or any error message.
>
> You just need to have the `requests` library installed (`pip install requests`) to run this script. Let me know if you > want any adjustments or further explanation!

---

## Appendix

### Kodit Request/Response from Cursor

#### Request

```json
{
  "user_intent": "Write a Python script to access a secret API at http://localhost:8000/secret.",
  "related_file_paths": [
    "temp.py"
  ],
  "related_file_contents": [
    ""
  ],
  "keywords": [
    "Python",
    "API request",
    "localhost",
    "secret",
    "http://localhost:8000/secret"
  ]
}
```

#### Response

> file:///Users/phil/.kodit/clones/https___gist.github.com_cbf0bd1f3338ddf9f98879148d2d752d.git/knock-knock-server.py
> This Python code snippet sets up a basic HTTP server that listens on a specified port (default is 8000) using the > `HTTPServer` class from the `http.server` module. The server utilizes a custom request handler called `KnockAuthHandler`, > which likely handles authentication through a "knock" method. The `run_server` function outputs instructions to the > console (using the `rich` library for formatted text) on how to access a secret area of the server. Users are required to > make three POST requests to the `/knock` endpoint, receive a token, and then use that token in an Authorization header to > make a GET request to the `/secret` endpoint. The server will continue to run indefinitely until it is stopped.
>
> ```
> import json
> import secrets
> from http.server import BaseHTTPRequestHandler, HTTPServer
> from rich.console import Console
> def run_server(port=8000):
>     server_address = ("", port)
>     httpd = HTTPServer(server_address, KnockAuthHandler)
>     console.print(f"[bold blue]Starting server on port {port}[/bold blue]")
>     console.print("[yellow]To access the secret area:[/yellow]")
>     console.print("1. Make 3 POST requests to /knock")
>     console.print("2. Use the returned token in the Authorization header")
>     console.print("3. Make a GET request to /secret with the token")
>     httpd.serve_forever()
> ```
>
> file:///Users/phil/.kodit/clones/https___gist.github.com_cbf0bd1f3338ddf9f98879148d2d752d.git/knock-knock-server.py
> This code defines a basic HTTP server in Python that responds to POST requests at the "/knock" endpoint. It includes a > class `KnockAuthHandler` that extends `BaseHTTPRequestHandler` to handle incoming requests.
>
> Here's a breakdown of the main components:
>
> 1. **Imports**: The code imports necessary modules, including `json` for encoding and decoding JSON data, `secrets` for > generating secure random tokens, `BaseHTTPRequestHandler` and `HTTPServer` for creating the HTTP server, and `rich.> console.Console` for formatted console output.
>
> 2. **do_POST Method**: This method handles POST requests:
>    - It checks if the request path is "/knock". If so:
>      - It attempts to retrieve a token from the request header using the `_get_token_from_header` method (which is not > defined in the snippet).
>      - If no token is found, a new secure token is created using `secrets.token_urlsafe(16)` and initialized in the > `knock_counts` dictionary with a count of 0.
>      - The count for the token is incremented, and the number of knocks received is printed to the console in green.
>      - A JSON response is constructed that includes a message about the number of knocks received, the token, and how > many knocks remain before a limit (set to 3).
>    - If the request path is anything other than "/knock", it returns a 404 error with a JSON response indicating that the > endpoint was not found.
>
> 3. **Response Handling**: The `_set_headers` method is called to set appropriate HTTP headers before sending back the > JSON response.
>
> Overall, this snippet sets up a simple knocking authentication system, allowing clients to "knock" by sending POST > requests to a specified endpoint while maintaining a count of knocks per token.
>
> ```
> import json
> import secrets
> from http.server import BaseHTTPRequestHandler, HTTPServer
> from rich.console import Console
> class KnockAuthHandler(BaseHTTPRequestHandler):
>     def do_POST(self):
>         if self.path == "/knock":
>             token = self._get_token_from_header()
>             if not token:
>                 token = secrets.token_urlsafe(16)
>                 knock_counts[token] = 0
> 
>             knock_counts[token] += 1
>             console.print(
>                 f"[green]Knock {knock_counts[token]} received for token {token}[/green]"
>             )
> 
>             response = {
>                 "message": f"Knock {knock_counts[token]} received",
>                 "token": token,
>                 "knocks_remaining": max(0, 3 - knock_counts[token]),
>             }
> 
>             self._set_headers()
>             self.wfile.write(json.dumps(response).encode())
>         else:
>             self._set_headers(404)
>             self.wfile.write(json.dumps({"error": "Not found"}).encode())
> ```
>
> file:///Users/phil/.kodit/clones/https___gist.github.com_cbf0bd1f3338ddf9f98879148d2d752d.git/knock-knock-server.py
> This code snippet defines a simple HTTP server using Python's `http.server` module and a custom request handler named > `KnockAuthHandler`.
>
> Here's a breakdown of the functionality:
>
> - The server listens for GET requests.
> - When a request is made to the path `/secret`, it checks for a token in the request headers.
> - The token is validated against a `knock_counts` dictionary, which tracks the number of times the token has been > "knocked" (i.e., how many requests have been made).
> - If the provided token is not present or if the knock count is less than 3, it responds with a 401 Unauthorized error > along with a message indicating that three knocks are required.
> - If the token is valid and the count is at least 3, it responds with a 200 status and welcomes the user to the secret > area, revealing a hidden message.
> - For any other paths, it returns a 404 Not Found error.
>
> Additional features include using JSON for responses and a potential use of the `rich` library for improved console > output (though it is not utilized in this snippet). The `secrets` module is imported but not used in the provided code.
>
> ```
> import json
> import secrets
> from http.server import BaseHTTPRequestHandler, HTTPServer
> from rich.console import Console
> class KnockAuthHandler(BaseHTTPRequestHandler):
>     def do_GET(self):
>         if self.path == "/secret":
>             token = self._get_token_from_header()
>             if not token or knock_counts.get(token, 0) < 3:
>                 self._set_headers(401)
>                 self.wfile.write(
>                     json.dumps(
>                         {
>                             "error": "Unauthorized",
>                             "message": "You need to knock three times first!",
>                         }
>                     ).encode()
>                 )
>                 return
> 
>             self._set_headers()
>             self.wfile.write(
>                 json.dumps(
>                     {
>                         "message": "Welcome to the secret area!",
>                         "secret": "The answer is 42",
>                     }
>                 ).encode()
>             )
>         else:
>             self._set_headers(404)
>             self.wfile.write(json.dumps({"error": "Not found"}).encode())
> ```
>
> file:///Users/phil/.kodit/clones/https___gist.github.com_cbf0bd1f3338ddf9f98879148d2d752d.git/knock-knock-server.py
> This code snippet imports necessary libraries and defines a class `KnockAuthHandler`, which extends > `BaseHTTPRequestHandler` from the `http.server` module. The class includes a method `_set_headers` that sets the HTTP > response status code (defaulting to 200) and specifies the content type of the response as "application/json". This > method prepares the server to send a JSON response for HTTP requests.
>
> The other imported modules, `json` and `secrets`, suggest that the class may be working with JSON data and generating > secure tokens or identifiers, although the snippet does not currently use them. The `rich.console` import indicates that > the script may eventually use the Rich library for enhanced console output, though it is not utilized in the provided > code.
>
> ```
> import json
> import secrets
> from http.server import BaseHTTPRequestHandler, HTTPServer
> from rich.console import Console
> class KnockAuthHandler(BaseHTTPRequestHandler):
>     def _set_headers(self, status_code=200):
>         self.send_response(status_code)
>         self.send_header("Content-type", "application/json")
>         self.end_headers()
> ```
>
> file:///Users/phil/.kodit/clones/https___gist.github.com_cbf0bd1f3338ddf9f98879148d2d752d.git/knock-knock-server.py
> The provided code snippet is a Python definition of a class `KnockAuthHandler` that extends `BaseHTTPRequestHandler` from > the `http.server` module. This class is designed to handle HTTP requests, specifically for authentication purposes.
>
> In this class:
>
> - The method `_get_token_from_header` retrieves the "Authorization" header from the HTTP request.
> - It checks if the header starts with the prefix "Bearer " and, if so, splits the string to extract the token part (the > portion after "Bearer ").
> - If the "Authorization" header does not contain a valid Bearer token, the method returns an empty string.
>
> The use of `json`, `secrets`, and `rich.console.Console` indicates that there may be additional functionality related to > JSON handling, secure random number generation, and enhanced console output, respectively, though these elements are not > utilized in the provided snippet.
>
> ```
> import json
> import secrets
> from http.server import BaseHTTPRequestHandler, HTTPServer
> from rich.console import Console
> class KnockAuthHandler(BaseHTTPRequestHandler):
>     def _get_token_from_header(self) -> str:
>         auth_header = self.headers.get("Authorization", "")
>         if auth_header.startswith("Bearer "):
>             return auth_header.split(" ")[1]
>         return ""
> ```

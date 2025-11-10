---
title: "Kodit Demo: Proprietary Auth Patterns"
linkTitle: Proprietary Code
description: A demo of Kodit where knowledge of proprietary code is required.
weight: 2
tags:
- demo
- auth
- cursor
---

A common feature of private codebases is proprietary code to achieve a certain
task. For example, you might a specific way to request data from your data lake, or a
specific format to use when using a common API.

This example uses the analogy of a proprietary authentication mechanism. I have a super
secret server with sensitive data. It is protected with a [custom auth implementation](https://gist.github.com/cbf0bd1f3338ddf9f98879148d2d752d)
that requires the user to "knock three times" before they get access.

You'll see that with Kodit the AI assistant delivers:

- Faster overall implementation
- Fewer mistakes
- Fewer tokens used

## Initial Results With Cursor

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

## Results When Adding Kodit to Cursor

Now let's index the code for my secret server and then leverage Kodit in Cursor.

1. (Optional) Make sure the [Kodit server is running](/kodit/getting-started/_index.md) and start afresh.
2. Index the [server
   code](https://gist.github.com/philwinder/cbf0bd1f3338ddf9f98879148d2d752d) via curl:

   ```sh
   curl --request POST \
   --url http://localhost:8080/api/v1/indexes \
   --header 'Content-Type: application/json' \
   --data '{
   "data": {
      "type": "index",
      "attributes": {
         "uri": "https://gist.github.com/philwinder/cbf0bd1f3338ddf9f98879148d2d752d.git"
      }
   }
   }'
   ```

   Wait for indexing to complete:

   ```sh
   curl --request GET \
   --url http://localhost:8080/api/v1/repositories/1/status \
   ```

3. [Connect Cursor](/kodit/getting-started/integration/index.md).

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

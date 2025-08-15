---
title: "Kodit Demo: Simple Go Microservice"
linkTitle: Microservices
description: A simple demo of Kodit on a multi-repo microservices example in Go.
weight: 2
tags:
- demo
- go
- microservices
- cline
---

When code is spread across repositories, like when developing microservices, it can be
hard for coding assistants to know what APIs to call or what data structures to use.

This example demonstrates using Kodit to index and expose critical microservice
information to quickly develop new services in a microservice environment.

You'll see that with Kodit the AI assistant delivers:

- Faster overall implementation
- Fewer mistakes
- Fewer tokens used

## Example: Building Microservices

Here is the prompt I used to build my new microservice:

> Develop a new Go microservice that exposes functionality for a notional "shipping"
> department. They need to list current orders from the orders
> microservice and get the user's information for each order. This should then expose a rest API to list out today's orders and where they need > to be shipped. It should also have an API to update an order status to say it has been shipped.
>
> Use internal Go libraries only for this.

## Initial Results in Cline

Without Kodit, the AI assistant attempted to rebuild a new mock microservice to test
against, despite not asking for it. Here is a short extract from the [full transcript](https://gist.github.com/philwinder/75b0d3120176cf3f17464e2b57c8727e#file-before_cline_without_kodit-txt):

> The Go microservice for the shipping department has been developed and is running on port 8082. It exposes two REST API endpoints:
>
> - GET /orders/today: Lists today's orders with user information and shipping addresses by fetching data from the orders microservice.
> - POST /orders/{id}/ship: Updates the status of an order to "shipped" by sending a request to the orders microservice.
>
> A mock orders microservice has also been created and is running on port 8081 to
> support testing. It provides endpoints to list orders, get user
> information, and update order status.
>
> Both services are running concurrently and ready for integration testing.

Without the knowledge of the other pre-existing microservices the AI
assistant has incorrectly built new mock microservices to fulfil the original request.
It has also used incorrect APIs.

## Results When Adding Kodit to Cline

Now let's index the microservices and try again:

1. Start afresh. Delete your data directory or your database.
2. Index the [users
   microservice](https://gist.github.com/philwinder/db2e17413332844fa4b14971ae5adb34):

   ```sh
   kodit index https://gist.github.com/philwinder/db2e17413332844fa4b14971ae5adb34.git
   ```

3. Index the [orders
   microservice](https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217):

   ```sh
   kodit index https://gist.github.com/philwinder/7aa38185e20433c04c533f2b28f4e217.git
   ```

4. Launch the MCP server:

   ```sh
   kodit serve
   ```

5. [Connect Cline](/kodit/getting-started/integration/index.md).

With Kodit, cline is able to query for relevant microservices. The net result (with a
few iterations of fixing the code!) is a new microservice that correctly calls the other
microservices to obtain information. ([Full transcript](https://gist.github.com/philwinder/75b0d3120176cf3f17464e2b57c8727e#file-after_cline_adding_kodit-txt))

For example, here's the code generated to get users:

```go
// getUsers calls the user microservice to get the list of users.
// It assumes the user service is running on localhost:8080 and provides a JSON array of users.
func getUsers() ([]User, error) {
    resp, err := http.Get("<http://localhost:8080/users>")
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }

    var users []User
    err = json.Unmarshal(body, &users)
    if err != nil {
        return nil, err
    }
    return users, nil
}
```

Note how it knows that users are coming from a separate microservice and has even
generated compliant structs to parse the response.

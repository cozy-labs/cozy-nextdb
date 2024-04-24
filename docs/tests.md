# Run automated tests

The automated tests can be launched with `make test`. It requires Docker (or
equivalent), as we use [testcontainers](https://testcontainers.com/) for
sandboxing the test databases. By default, the tests are using a postgres 16
docker image, but you can use another image like this:

```sh
$ TEST_NEXTDB_PG_IMAGE=docker.io/postgres-15-alpine go test ./web
```

It is possible to see the SQL queries in the output by using the debug log level:

```sh
$ TEST_NEXTDB_LOG_LEVEL=debug go test ./web
```

## Tracing

It can be useful to know where the time is passed in order to optimize, and
tracing is a tool that can help that. We can run the tests with golang tracing
enabled, and then look at the trace in a browser:

```sh
$ go test ./web -trace=trace.out
$ go tool trace trace.out
```

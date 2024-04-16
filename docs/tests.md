# Run automated tests

The automated tests can be launched with `make test`. It requires Docker (or
equivalent), as we use [testcontainers](https://testcontainers.com/) for
sandboxing the test databases.

It is possible to see the SQL queries in the output by using the debug log level:

```sh
$ TEST_NEXTDB_LOG_LEVEL=debug go test ./web
```

# Configuration

There are 3 ways to configure cozy-nextdb:

1. Environment variables
2. Flags on the command line
3. Configuration file

For example, the port on which cozy-nextdb will listen for HTTP requests is
configurable via the `COZY_NEXTDB_PORT` env variable, via the `--port` option of
`cozy-nextdb serve`, or via the `port:` entry of the configuration file.

The configuration file is the recommended way, and it is the only way to
configure advanced features.

## Flags on the command line

The flags are documented in the [man pages](cli/cozy-nextdb.md).

## Environment variables

The environment variables are upcased, with a `COZY_NEXTDB_` prefix.

## Configuration file

A file named [`nextdb.example.yaml`](../nextdb.example.yaml) can be used as an
example of the configuration file. It lists the available configuration
parameters and give some hints in comments.

If you need to edit the configuration, we recommend to only copy the needed
part in a new file. This new file should be named `nextdb.yaml`, `nextdb.yml`,
or `nextdb.json` depending on the format of your chosing, and should be present
in one of these directories (ordered by priority):

- `.`
- `$HOME/.cozy`
- `/etc/cozy`

The path of the configuration file can also be define from an absolute path
given by the `--config` (or `-c`) flag of the [cozy-nextdb
command](cli/cozy-nextdb_serve.md).

## HTTPS

The cozy-nextdb can be configured to use TLS. It has been tested in local with
those commands:

```sh
$ openssl genrsa -out server.key 2048
$ openssl req -new -x509 -sha256 -key server.key -out server.pem -days 365 -subj "/C=FR/ST=France/L=Paris/O=CozyCloud/CN=localhost"
$ cozy-nextdb serve --cert-file server.pem --key-file server.key
$ curl -v --cacert server.pem https://localhost:7654/status
```

## Logs

### Levels

We have used 4 log levels in cozy-nextdb:

- `error` is when something unexpected happens (aka a bug), it means that a
  developper should look at it
- `warn` is important messages, like PostgreSQL unavailable, errors
  in a request format from a client, job failures, cannot bind port, etc.
- `info` is for logging what the cozy-nextdb does, like logging process
  starts/stops, HTTP requests, jobs, etc.
- `debug` is for debugging, and it includes the requests to PostgreSQL.

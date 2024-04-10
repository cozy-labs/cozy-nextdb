## cozy-nextdb serve

Starts the dedup and listens for HTTP calls

### Synopsis

Starts the dedup and listens for HTTP calls
It will accept HTTP requests on localhost:9000 by default.
Use the --port and --host flags to change the listening option.

```
cozy-nextdb serve [flags]
```

### Options

```
      --cert-file string   the certificate file for TLS
  -h, --help               help for serve
  -H, --host string        server host (default "localhost")
      --key-file string    the key file for TLS
  -p, --port int           server port (default 7654)
```

### Options inherited from parent commands

```
  -c, --config string      path to the configuration file
  -L, --log-level string   set the logger level (default "info")
      --log-syslog         use the local syslog for logging
  -d, --pg-url string      set the URL of the PostgreSQL server (default "postgres://nextdb:nextdb@localhost:5432/nextdb")
```

### SEE ALSO

* [cozy-nextdb](cozy-nextdb.md)	 - cozy-nextdb is the main command


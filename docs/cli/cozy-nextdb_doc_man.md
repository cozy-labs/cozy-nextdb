## cozy-nextdb doc man

Print the manpages of cozy-nextdb

### Synopsis

Print the manual pages for using cozy-nextdb in command-line

```
cozy-nextdb doc man <directory> [flags]
```

### Examples

```
$ mkdir -p ~/share/man
$ export MANPATH=~/share/man:$MANPATH
$ cozy-nextdb doc man ~/share/man
$ man cozy-nextdb
```

### Options

```
  -h, --help   help for man
```

### Options inherited from parent commands

```
  -c, --config string      path to the configuration file
  -L, --log-level string   set the logger level (default "info")
      --log-syslog         use the local syslog for logging
  -d, --pg-url string      set the URL of the PostgreSQL server (default "postgres://nextdb:nextdb@localhost:5432/nextdb")
```

### SEE ALSO

* [cozy-nextdb doc](cozy-nextdb_doc.md)	 - Print the documentation


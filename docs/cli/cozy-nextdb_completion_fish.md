## cozy-nextdb completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	cozy-nextdb completion fish | source

To load completions for every new session, execute once:

	cozy-nextdb completion fish > ~/.config/fish/completions/cozy-nextdb.fish

You will need to start a new shell for this setup to take effect.


```
cozy-nextdb completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
  -c, --config string      path to the configuration file
  -L, --log-level string   set the logger level (default "info")
      --log-syslog         use the local syslog for logging
  -d, --pg-url string      set the URL of the PostgreSQL server (default "postgres://nextdb:nextdb@localhost:5432/nextdb")
```

### SEE ALSO

* [cozy-nextdb completion](cozy-nextdb_completion.md)	 - Generate the autocompletion script for the specified shell


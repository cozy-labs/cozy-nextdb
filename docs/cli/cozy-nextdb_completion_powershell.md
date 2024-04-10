## cozy-nextdb completion powershell

Generate the autocompletion script for powershell

### Synopsis

Generate the autocompletion script for powershell.

To load completions in your current shell session:

	cozy-nextdb completion powershell | Out-String | Invoke-Expression

To load completions for every new session, add the output of the above command
to your powershell profile.


```
cozy-nextdb completion powershell [flags]
```

### Options

```
  -h, --help              help for powershell
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


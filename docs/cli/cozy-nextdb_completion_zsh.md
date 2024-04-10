## cozy-nextdb completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(cozy-nextdb completion zsh)

To load completions for every new session, execute once:

#### Linux:

	cozy-nextdb completion zsh > "${fpath[1]}/_cozy-nextdb"

#### macOS:

	cozy-nextdb completion zsh > $(brew --prefix)/share/zsh/site-functions/_cozy-nextdb

You will need to start a new shell for this setup to take effect.


```
cozy-nextdb completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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


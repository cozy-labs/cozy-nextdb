## cozy-nextdb completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(cozy-nextdb completion bash)

To load completions for every new session, execute once:

#### Linux:

	cozy-nextdb completion bash > /etc/bash_completion.d/cozy-nextdb

#### macOS:

	cozy-nextdb completion bash > $(brew --prefix)/etc/bash_completion.d/cozy-nextdb

You will need to start a new shell for this setup to take effect.


```
cozy-nextdb completion bash
```

### Options

```
  -h, --help              help for bash
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


# Install a local environement for development

## Dependencies

You will need Go 1.22, git, make and curl.

```sh
$ git clone git@github.com:cozy-labs/cozy-nextdb.git
$ cd cozy-nextdb
```

## PostgreSQL

Install postgresql and create a database, with a user that can use it:

```sh
# apt install postgresql postgresql-contrib
# sudo -i -u postgres
$ psql
> CREATE DATABASE nextdb;
> CREATE USER nextdb WITH PASSWORD 'nextdb';
> GRANT ALL PRIVILEGES ON DATABASE nextdb TO nextdb;
```

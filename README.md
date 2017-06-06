# CopyQL

**tl;dr** Copies data and related data from one SQL store to another. Includes an intermediate JSON format!

## Usage

In order to use CopyQL, you must have a YAML configuration in your working directory `copyql.yaml`. This defines things like custom relationships and the connections between your remote and local data stores.

### `copyql.yaml`

* `source|destination` - defines the SQL connections
  * `host` - the host of the database (defaults to `localhost`)
  * `port` - the port of the database (defaults to `3306`)
  * `user` - the connecting user (defaults to `root`)
  * `password` - the password for the user
  * `database` - the database to use when connecting (required)

### Command

```
copyql [options] <table>.<column>:<value>
```

> When copying, it is required to provide an entry point. This could be something like `libraries.id:1234` and it will copy the library with the id `1234` and all related data, including `books` and their related `authors`.

#### Options

* `--out` - a JSON file to dump the contents. If this option is specified, no data will be copied to the destination SQL data store.
* `--in` - a JSON file to read the contents from. If this option is specified, data will not be gathered from the source database.
* `--config, -c` - the location of a custom configuration file. Defaults to `copyql.yaml` in the working directory.

## Purpose

It is often useful to have real (scrubbed) data when developing, adding features, and performance testing. Having this data locally can greatly speed up development time. However, in bigger systems, the data store is generally too large to copy the entire store locally. 

That's where **CopyQL** comes in.

CopyQL builds relationships based on manual configuration and automatic naming conventions. Below is an example of the relationships it looks for automatically.

```
+-----------+     +------------+     +---------+
| libraries <--+  | books      |  +--> authors |
+-----------+  |  +------------+  |  +---------+
| id        |  |  | id         |  |  | id      |
| name      |  +--+ library_id |  |  | name    |
| owner     |     | title      |  |  +---------+
+-----------+     | author_id  +--+
                  +------------+
```

## Contributing

We welcome pull requests and suggestions! 👍

## Todo

* [ ] Add manual relationship configuration
* [ ] Improve data inserting to avoid column not found and duplicate key errors
* [ ] Add ability to set automatic relationship patterns
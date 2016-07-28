[![GoDoc](https://godoc.org/gopkg.in/sameervitian/sqlt.v1?status.svg)](https://godoc.org/gopkg.in/sameervitian/sqlt.v1)

#sqlt

sqlt is a wrapper package for `jmoiron/sqlx`

this wrapper build based on `tsenart/nap` master-slave configuration with some modification

since this package is just a wrapper, you can use it 100% like `sqlx`, but with some differences

Usage
------

to connect to database, you need an appended connection string with `;` delimeter, but there is some notes:
* the first connection will always be considered as `master` connection
* another connection will be considered as `slave`

```go
databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.Open("postgres", databaseCon)
```

or

```go
databaseCon := "con1"
db, err := sqlt.Open("postgres", databaseCon)
```

for a complete `sqlx` features, you can use this:

```go
err := db.Slave().Query(&struct, query)
err := db.Master().Query(&struct, query)
```

but if you don't want to state master or slave, you can use it like this:

```go
err := db.Query(&struct, query)
```

`preapre` and `preparex` for `sql` and `sqlx` are supported

use `preparex` to enable `scanStruct`

```go
statement := db.Prepare(query)
rows, err := statement.Query(param)
row := statement.QueryRows(param)
```

```go
statement := db.Preparex(query)
rows, err := statement.Query(param)
row := statement.QueryRows(param)
```

Heartbeat/Watcher
------

Use `DoHeartbeat` to auto-reconnect/watch your database connection.

```go
databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.Open("postgres", databaseCon)

if err != nil {
  return err
}

//this will automatically ping the database and watch the connection
db.DoHeartBeat()
```

You can also get the database status in JSON, for example:

```go
databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.OpenWithName("postgres", databaseCon, "order")

if err != nil {
  return err
}

//this will automatically ping the database and watch the connection
db.DoHeartBeat()

//this will return database status in JSON
status, _ := db.GetStatus()
```

JSON output:

```json
{
  "order": {
    "db_list": [
      {
      "name": "master",
      "connected": true,
      "last_active": "Mon, 04 Jul 2016 11:25:14 WIB",
      "error": null
      },
      {
      "name": "slave-1",
      "connected": true,
      "last_active": "Mon, 04 Jul 2016 11:25:14 WIB",
      "error": null
      },
      {
      "name": "slave-2",
      "connected": true,
      "last_active": "Mon, 04 Jul 2016 11:25:14 WIB",
      "error": null
      }
    ],
  }
}
```


----------------------------------

3rd party references:
* https://github.com/jmoiron/sqlx
* https://github.com/tsenart/nap

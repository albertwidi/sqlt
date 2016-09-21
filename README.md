[![GoDoc](https://godoc.org/gopkg.in/sameervitian/sqlt.v1?status.svg)](https://godoc.org/gopkg.in/sameervitian/sqlt.v1)

#sqlt

Sqlt is a wrapper package for `jmoiron/sqlx`

This wrapper build based on `tsenart/nap` master-slave and its `load-balancing` configuration with some modification

Since this package is just a wrapper, you can use it 100% like `sqlx`, but with some differences

Usage
------

To connect to database, you need an appended connection string with `;` delimeter, but there is some notes:
* First connection will always be considered as `master` connection
* Another connection will be considered as `slave`

```go
databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.Open("postgres", databaseCon)
```

Or

```go
databaseCon := "con1"
db, err := sqlt.Open("postgres", databaseCon)
```

Query Example:

```go
row := db.QuerRowy(query, args)
```

```go
rows, err := db.Query(query, args)
```

```go
err := db.Select(&struct, query, args)
```

```go
err := db.Get(&struct, query, args)
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

SQLT provide a mechanism to switch between slaves if something bad happen. If no slaves is available then all connection will go to master.

The watcher will auto-reconnect to your slaves if possible and start to `load-balancing` itself.

To watch all slaves connection, use `DoHeartbeat`. This will ping your database every second and make auto-reconnect if needed.

```go
databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.Open("postgres", databaseCon)

if err != nil {
  return err
}

//this will automatically ping the database and watch the connection
db.DoHeartBeat()
```

Database status
------

You can also get the database status, for example:

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

Output:

```go
DbStatus {
  Name:       "order",
  Connected:  true,
  LastActive: "21 September 2016",
  Error:      nil,
}
```


----------------------------------

3rd party references:
* https://github.com/jmoiron/sqlx
* https://github.com/tsenart/nap

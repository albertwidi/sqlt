#sqlt

sqlt is a wrapper package for `jmoiron/sqlx`

this wrapper build based on `tsenart/nap` master-slave configuration

since this package is just a wrapper, you can use it 100% like `sqlx`, but with some differences

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

for a complete sqlx features, you can use this:

```go
err := db.Slave().Query(&struct, query)
err := db.Master().Query(&struct, query)
```

but if you don't want to state master or slave, you can use it like this:

```go
err := db.Query(&struct, query)
```

straightforward operation like this is limited and not all features are ported into `sqlt`, for example: `statements`

please consider to use either `db.Slave` or `db.Master` for complex operations

----------------------------------

3rd party references:
* https://github.com/jmoiron/sqlx
* https://github.com/tsenart/nap

#sqlt

sqlt is a wrapper package for sqlx

this wrapper build based on `tsenart/nap` master-slave configuration

since this package is just a wrapper, you can use it 100% like sqlx, but with some differences

to connect to database, you need an appended connection string with `;` delimeter, but there is some note:
* the first connection will always considered as master connection
* another connection will considered as slave

```databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.Open("postgres", databaseCon)
```

or 

```databaseCon := "con1"
db, err := sqlt.Open("postgres", databaseCon)
```

for a complete sqlx features, you can use this:

```err := db.Slave().Query(&struct, query)
```

but if you don't want to state master or slave, you can use it like this:

```err := db.Query(&struct, query)
```

but this kind of operation is limited and not all features are ported into sqlt

----------------------------------

3rd party references:
*https://github.com/jmoiron/sqlx
*https://github.com/tsenart/nap
SQLT

SQLT is SQLX (jmoiron/sqlx) Wrapper

This is simply a wrapper of Master - Slave configuration based on 'tsenart/nap'

To Connect:

databaseCon := "con1;" + "con2;" + "con3"
db, err := sqlt.Open("postgres", databaseCon)

For a complete sqlx features, you can use this:

something := Something{}
err := db.Slave().Query(&something, query)

If you want a simple operation without call a slave or master you can also do this:

err := db.Query(&something, query)

----------------------------------

3rd party references:
https://github.com/jmoiron/sqlx
https://github.com/tsenart/nap
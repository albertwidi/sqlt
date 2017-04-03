package sqlt

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

func openContextConnection(ctx context.Context, driverName, sources string, groupName string) (*DB, error) {
	var err error

	conns := strings.Split(sources, ";")
	connsLength := len(conns)

	//check if no source is available
	if connsLength < 1 {
		return nil, errors.New("No sources found")
	}

	db := &DB{
		sqlxdb: make([]*sqlx.DB, connsLength),
		stats:  make([]DbStatus, connsLength),
	}
	db.length = connsLength
	db.driverName = driverName

	for i := range conns {
		db.sqlxdb[i], err = sqlx.Open(driverName, conns[i])
		if err != nil {
			db.inactivedb = append(db.inactivedb, i)
			return nil, err
		}
		constatus := true

		//set the name
		name := ""
		if i == 0 {
			name = "master"
		} else {
			name = "slave-" + strconv.Itoa(i)
		}

		status := DbStatus{
			Name:       name,
			Connected:  constatus,
			LastActive: time.Now().String(),
		}

		db.stats[i] = status
		db.activedb = append(db.activedb, i)
	}

	//set the default group name
	db.groupName = defaultGroupName
	if groupName != "" {
		db.groupName = groupName
	}

	//ping database to retrieve error
	err = db.PingContext(ctx)
	return db, err
}

// OpenWithContext opening connection with context
func OpenWithContext(ctx context.Context, driver, sources string) (*DB, error) {
	return openContextConnection(ctx, driver, sources, "")
}

//PingContext database
func (db *DB) PingContext(ctx context.Context) error {
	var err error

	if !db.heartBeat {
		return db.PingContext(ctx)
	}

	for i := 0; i < len(db.activedb); i++ {
		val := db.activedb[i]
		err = db.sqlxdb[val].PingContext(ctx)
		name := db.stats[val].Name

		if err != nil {
			if db.length <= 1 {
				return err
			}

			db.stats[val].Connected = false
			db.activedb = append(db.activedb[:i], db.activedb[i+1:]...)
			i--
			db.inactivedb = append(db.inactivedb, val)
			db.stats[val].Error = errors.New(name + ": " + err.Error())
			dbLengthMutex.Lock()
			db.length--
			dbLengthMutex.Unlock()
		} else {
			db.stats[val].Connected = true
			db.stats[val].LastActive = time.Now().Format(time.RFC1123)
			db.stats[val].Error = nil
		}
	}

	for i := 0; i < len(db.inactivedb); i++ {
		val := db.inactivedb[i]
		err = db.sqlxdb[val].PingContext(ctx)
		name := db.stats[val].Name

		if err != nil {
			db.stats[val].Connected = false
			db.stats[val].Error = errors.New(name + ": " + err.Error())
		} else {
			db.stats[val].Connected = true
			db.inactivedb = append(db.inactivedb[:i], db.inactivedb[i+1:]...)
			i--
			db.activedb = append(db.activedb, val)
			db.stats[val].LastActive = time.Now().Format(time.RFC1123)
			db.stats[val].Error = nil
			dbLengthMutex.Lock()
			db.length++
			dbLengthMutex.Unlock()
		}
	}
	return err
}

//SelectContext using slave db.
func (db *DB) SelectContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return db.sqlxdb[db.slave()].SelectContext(ctx, dest, query, args...)
}

//SelectMasterContext using master db.
func (db *DB) SelectMasterContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return db.sqlxdb[0].SelectContext(ctx, dest, query, args...)
}

//GetContext using slave.
func (db *DB) GetContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return db.sqlxdb[db.slave()].GetContext(ctx, dest, query, args...)
}

//GetMasterContext using master.
func (db *DB) GetMasterContext(ctx context.Context, dest interface{}, query string, args ...interface{}) error {
	return db.sqlxdb[0].GetContext(ctx, dest, query, args...)
}

//PrepareContext return sql stmt
func (db *DB) PrepareContext(ctx context.Context, query string) (Stmt, error) {
	var err error
	stmt := Stmt{}
	stmts := make([]*sql.Stmt, len(db.sqlxdb))

	for i := range db.sqlxdb {
		stmts[i], err = db.sqlxdb[i].PrepareContext(ctx, query)

		if err != nil {
			return stmt, err
		}
	}
	stmt.db = db
	stmt.stmts = stmts
	return stmt, nil
}

//PreparexContext sqlx stmt
func (db *DB) PreparexContext(ctx context.Context, query string) (*Stmtx, error) {
	var err error
	stmts := make([]*sqlx.Stmt, len(db.sqlxdb))

	for i := range db.sqlxdb {
		stmts[i], err = db.sqlxdb[i].PreparexContext(ctx, query)

		if err != nil {
			return nil, err
		}
	}

	return &Stmtx{db: db, stmts: stmts}, nil
}

// QueryContext queries the database and returns an *sql.Rows.
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	r, err := db.sqlxdb[db.slave()].QueryContext(ctx, query, args...)
	return r, err
}

//QueryRowContext queries the database and returns an *sqlx.Row.
func (db *DB) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	rows := db.sqlxdb[db.slave()].QueryRowContext(ctx, query, args...)
	return rows
}

//QueryxContext queries the database and returns an *sqlx.Rows.
func (db *DB) QueryxContext(ctx context.Context, query string, args ...interface{}) (*sqlx.Rows, error) {
	r, err := db.sqlxdb[db.slave()].QueryxContext(ctx, query, args...)
	return r, err
}

// QueryRowxContext queries the database and returns an *sqlx.Row.
func (db *DB) QueryRowxContext(ctx context.Context, query string, args ...interface{}) *sqlx.Row {
	rows := db.sqlxdb[db.slave()].QueryRowxContext(ctx, query, args...)
	return rows
}

//ExecContext using master db
func (db *DB) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return db.sqlxdb[0].ExecContext(ctx, query, args...)
}

// MustExecContext (panic) runs MustExec using master database.
func (db *DB) MustExecContext(ctx context.Context, query string, args ...interface{}) sql.Result {
	return db.sqlxdb[0].MustExecContext(ctx, query, args...)
}

//ExecContext will always go to production
func (st *Stmtx) ExecContext(ctx context.Context, args ...interface{}) (sql.Result, error) {
	return st.stmts[0].ExecContext(ctx, args...)
}

//QueryContext will always go to slave
func (st *Stmtx) QueryContext(ctx context.Context, args ...interface{}) (*sql.Rows, error) {
	return st.stmts[st.db.slave()].QueryContext(ctx, args...)
}

//QueryMasterContext will use master db
func (st *Stmtx) QueryMasterContext(ctx context.Context, args ...interface{}) (*sql.Rows, error) {
	return st.stmts[0].QueryContext(ctx, args...)
}

//QueryRowContext will always go to slave
func (st *Stmtx) QueryRowContext(ctx context.Context, args ...interface{}) *sql.Row {
	return st.stmts[st.db.slave()].QueryRowContext(ctx, args...)
}

//QueryRowMasterContext will use master db
func (st *Stmtx) QueryRowMasterContext(ctx context.Context, args ...interface{}) *sql.Row {
	return st.stmts[0].QueryRowContext(ctx, args...)
}

//MustExecContext using master database
func (st *Stmtx) MustExecContext(ctx context.Context, args ...interface{}) sql.Result {
	return st.stmts[0].MustExecContext(ctx, args...)
}

//QueryxContext will always go to slave
func (st *Stmtx) QueryxContext(ctx context.Context, args ...interface{}) (*sqlx.Rows, error) {
	return st.stmts[st.db.slave()].QueryxContext(ctx, args...)
}

// QueryRowxContext will always go to slave
func (st *Stmtx) QueryRowxContext(ctx context.Context, args ...interface{}) *sqlx.Row {
	return st.stmts[st.db.slave()].QueryRowxContext(ctx, args...)
}

// QueryRowxMasterContext will always go to master
func (st *Stmtx) QueryRowxMasterContext(ctx context.Context, args ...interface{}) *sqlx.Row {
	return st.stmts[0].QueryRowxContext(ctx, args...)
}

// GetContext will always go to slave
func (st *Stmtx) GetContext(ctx context.Context, dest interface{}, args ...interface{}) error {
	return st.stmts[st.db.slave()].GetContext(ctx, dest, args...)
}

// GetMasterContext will always go to master
func (st *Stmtx) GetMasterContext(ctx context.Context, dest interface{}, args ...interface{}) error {
	return st.stmts[0].GetContext(ctx, dest, args...)
}

// SelectContext will always go to slave
func (st *Stmtx) SelectContext(ctx context.Context, dest interface{}, args ...interface{}) error {
	return st.stmts[st.db.slave()].SelectContext(ctx, dest, args...)
}

// SelectMasterContext will always go to master
func (st *Stmtx) SelectMasterContext(ctx context.Context, dest interface{}, args ...interface{}) error {
	return st.stmts[0].SelectContext(ctx, dest, args...)
}

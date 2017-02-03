package sqlt

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jmoiron/sqlx"
)

var (
	ErrNoConnectionDetected = errors.New("No connection detected")
)

//DB struct wrapper for sqlx connection
type DB struct {
	sqlxdb     []*sqlx.DB
	activedb   []int
	inactivedb []int
	driverName string
	groupName  string
	length     int
	count      uint64
	//for stats
	stats     []DbStatus
	heartbeat bool
	lastBeat  string
}

//DbStatus for status response
type DbStatus struct {
	Name       string      `json:"name"`
	Connected  bool        `json:"connected"`
	LastActive string      `json:"last_active"`
	Error      interface{} `json:"error"`
}

type statusResponse struct {
	Dbs       interface{} `json:"db_list"`
	Heartbeat bool        `json:"heartbeat"`
	Lastbeat  string      `json:"last_beat"`
}

const defaultGroupName = "sqlt_open"

var dbLengthMutex = &sync.Mutex{}

//for persistent functions
var maxRetry int

func openConnection(driverName, sources string, groupName string) (*DB, error) {
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
	err = db.Ping()

	//set maxRetry default value
	maxRetry = 1
	return db, err
}

//Open connection to database
func Open(driverName, sources string) (*DB, error) {
	return openConnection(driverName, sources, "")
}

//OpenWithName open the connection and set connection group name
func OpenWithName(driverName, sources string, name string) (*DB, error) {
	return openConnection(driverName, sources, name)
}

//GetStatus return database status
func (db *DB) GetStatus() ([]DbStatus, error) {
	if len(db.stats) == 0 {
		return db.stats, ErrNoConnectionDetected
	}

	//if heartbeat is not enabled, ping to get status before send status
	if !db.heartbeat {
		db.Ping()
	}

	return db.stats, nil
}

//DoHeartBeat will automatically spawn a goroutines to ping your database every one second, use this carefully
func (db *DB) DoHeartBeat() {
	if !db.heartbeat {
		go func() {
			for range time.Tick(time.Second * 1) {
				db.Ping()
				db.lastBeat = time.Now().Format(time.RFC1123)
			}
		}()
	}

	db.heartbeat = true
}

//Ping database
func (db *DB) Ping() error {
	var err error

	for i := 0; i < len(db.activedb); i++ {
		val := db.activedb[i]
		err = db.sqlxdb[val].Ping()
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
		err = db.sqlxdb[val].Ping()
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

//Prepare return sql stmt
func (db *DB) Prepare(query string) (Stmt, error) {
	var err error
	stmt := Stmt{}
	stmts := make([]*sql.Stmt, len(db.sqlxdb))

	for i := range db.sqlxdb {
		stmts[i], err = db.sqlxdb[i].Prepare(query)

		if err != nil {
			return stmt, err
		}
	}

	stmt.db = db
	stmt.stmts = stmts
	return stmt, nil
}

//Preparex sqlx stmt
func (db *DB) Preparex(query string) (*Stmtx, error) {
	var err error
	stmts := make([]*sqlx.Stmt, len(db.sqlxdb))

	for i := range db.sqlxdb {
		stmts[i], err = db.sqlxdb[i].Preparex(query)

		if err != nil {
			return nil, err
		}
	}

	return &Stmtx{db: db, stmts: stmts}, nil
}

//SetMaxOpenConnections to set max connections
func (db *DB) SetMaxOpenConnections(max int) {
	for i := range db.sqlxdb {
		db.sqlxdb[i].SetMaxOpenConns(max)
	}
}

// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
// Expired connections may be closed lazily before reuse.
// If d <= 0, connections are reused forever.
func (db *DB) SetConnMaxLifetime(d time.Duration) {
	for i := range db.sqlxdb {
		db.sqlxdb[i].SetConnMaxLifetime(d)
	}
}

//Slave return slave database
func (db *DB) Slave() *sqlx.DB {
	return db.sqlxdb[db.slave()]
}

//Master return master database
func (db *DB) Master() *sqlx.DB {
	return db.sqlxdb[0]
}

// Query queries the database and returns an *sql.Rows.
func (db *DB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	r, err := db.sqlxdb[db.slave()].Query(query, args...)
	return r, err
}

// PersistentQuery queries the database persistently and returns an *sql.Rows.
func (db *DB) PersistentQuery(query string, args ...interface{}) (*sql.Rows, error) {
	r, err := db.sqlxdb[db.slave()].Query(query, args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			r, err := db.sqlxdb[db.slave()].Query(query, args...)
			if err == nil {
				return r, err
			}
		}
	}
	return r, err
}

//QueryRow queries the database and returns an *sqlx.Row.
func (db *DB) QueryRow(query string, args ...interface{}) *sql.Row {
	rows := db.sqlxdb[db.slave()].QueryRow(query, args...)
	return rows
}

//Queryx queries the database and returns an *sqlx.Rows.
func (db *DB) Queryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	r, err := db.sqlxdb[db.slave()].Queryx(query, args...)
	return r, err
}

//PersistentQueryx queries the database persistently and returns an *sqlx.Rows. (persistently)
func (db *DB) PersistentQueryx(query string, args ...interface{}) (*sqlx.Rows, error) {
	r, err := db.sqlxdb[db.slave()].Queryx(query, args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			r, err := db.sqlxdb[db.slave()].Queryx(query, args...)
			if err == nil {
				return r, err
			}
		}
	}
	return r, err
}

// QueryRowx queries the database and returns an *sqlx.Row.
func (db *DB) QueryRowx(query string, args ...interface{}) *sqlx.Row {
	rows := db.sqlxdb[db.slave()].QueryRowx(query, args...)
	return rows
}

//Exec using master db
func (db *DB) Exec(query string, args ...interface{}) (sql.Result, error) {
	return db.sqlxdb[0].Exec(query, args...)
}

// MustExec (panic) runs MustExec using master database.
func (db *DB) MustExec(query string, args ...interface{}) sql.Result {
	return db.sqlxdb[0].MustExec(query, args...)
}

//Select using slave db.
func (db *DB) Select(dest interface{}, query string, args ...interface{}) error {
	return db.sqlxdb[db.slave()].Select(dest, query, args...)
}

//PersistentSelect using this DB persistently.
func (db *DB) PersistentSelect(dest interface{}, query string, args ...interface{}) error {
	err := db.sqlxdb[db.slave()].Select(dest, query, args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			err := db.sqlxdb[db.slave()].Select(dest, query, args...)
			if err == nil {
				return err
			}
		}
	}
	return err
}

//Get using slave.
func (db *DB) Get(dest interface{}, query string, args ...interface{}) error {
	return db.sqlxdb[db.slave()].Get(dest, query, args...)
}

//PersistentGet using slave DB persistently.
func (db *DB) PersistentGet(dest interface{}, query string, args ...interface{}) error {
	err := db.sqlxdb[db.slave()].Get(dest, query, args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			err := db.sqlxdb[db.slave()].Get(dest, query, args...)
			if err == nil {
				return err
			}
		}
	}

	return err
}

// NamedExec using master db.
func (db *DB) NamedExec(query string, arg interface{}) (sql.Result, error) {
	return db.sqlxdb[0].NamedExec(query, arg)
}

// Transaction
func (db *DB) Begin() (*sql.Tx, error) {
	return db.sqlxdb[0].Begin()
}

func (db *DB) Beginx() (*sqlx.Tx, error) {
	return db.sqlxdb[0].Beginx()
}

//MustBegin starts a transaction, and panics on error.  Returns an *sqlx.Tx instead
//of an *sql.Tx.
func (db *DB) MustBegin() *sqlx.Tx {
	tx, err := db.sqlxdb[0].Beginx()
	if err != nil {
		panic(err)
	}
	return tx
}

// Sqlx Rebind
func (db *DB) Rebind(query string) string {
	return db.sqlxdb[db.slave()].Rebind(query)
}

func (db *DB) RebindMaster(query string) string {
	return db.sqlxdb[0].Rebind(query)
}

/*******************************************/

//Stmt implement sql stmt
type Stmt struct {
	db    *DB
	stmts []*sql.Stmt
}

//Exec will always go to production
func (st *Stmt) Exec(args ...interface{}) (sql.Result, error) {
	return st.stmts[0].Exec(args...)
}

//Query will always go to slave
func (st *Stmt) Query(args ...interface{}) (*sql.Rows, error) {
	return st.stmts[st.db.slave()].Query(args...)
}

//QueryMaster will use master db
func (st *Stmt) QueryMaster(args ...interface{}) (*sql.Rows, error) {
	return st.stmts[0].Query(args...)
}

//PersistentQuery will always go to slave persistently
func (st *Stmt) PersistentQuery(args ...interface{}) (*sql.Rows, error) {
	r, err := st.stmts[st.db.slave()].Query(args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			r, err := st.stmts[st.db.slave()].Query(args...)
			if err == nil {
				return r, err
			}
		}
	}
	return r, err
}

//QueryRow will always go to slave
func (st *Stmt) QueryRow(args ...interface{}) *sql.Row {
	return st.stmts[st.db.slave()].QueryRow(args...)
}

//QueryRowMaster will use master db
func (st *Stmt) QueryRowMaster(args ...interface{}) *sql.Row {
	return st.stmts[0].QueryRow(args...)
}

//Close stmt
func (st *Stmt) Close() error {
	for i := range st.stmts {
		err := st.stmts[i].Close()

		if err != nil {
			return err
		}
	}
	return nil
}

/********************************************/

//Stmtx implement sqlx stmt
type Stmtx struct {
	db    *DB
	stmts []*sqlx.Stmt
}

//Close all dbs connection
func (st *Stmtx) Close() error {
	for i := range st.stmts {
		err := st.stmts[i].Close()

		if err != nil {
			return err
		}
	}
	return nil
}

//Exec will always go to production
func (st *Stmtx) Exec(args ...interface{}) (sql.Result, error) {
	return st.stmts[0].Exec(args...)

}

//Query will always go to slave
func (st *Stmtx) Query(args ...interface{}) (*sql.Rows, error) {
	return st.stmts[st.db.slave()].Query(args...)
}

//QueryMaster will use master db
func (st *Stmtx) QueryMaster(args ...interface{}) (*sql.Rows, error) {
	return st.stmts[0].Query(args...)
}

//PersistentQuery will always go to slave persistently
func (st *Stmtx) PersistentQuery(args ...interface{}) (*sql.Rows, error) {
	r, err := st.stmts[st.db.slave()].Query(args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			r, err := st.stmts[st.db.slave()].Query(args...)
			if err == nil {
				return r, err
			}
		}
	}
	return r, err
}

//QueryRow will always go to slave
func (st *Stmtx) QueryRow(args ...interface{}) *sql.Row {
	return st.stmts[st.db.slave()].QueryRow(args...)
}

//QueryRowMaster will use master db
func (st *Stmtx) QueryRowMaster(args ...interface{}) *sql.Row {
	return st.stmts[0].QueryRow(args...)
}

//MustExec using master database
func (st *Stmtx) MustExec(args ...interface{}) sql.Result {
	return st.stmts[0].MustExec(args...)
}

//Queryx will always go to slave
func (st *Stmtx) Queryx(args ...interface{}) (*sqlx.Rows, error) {
	return st.stmts[st.db.slave()].Queryx(args...)
}

//PersistentQueryx will always go to slave persistently
func (st *Stmtx) PersistentQueryx(args ...interface{}) (*sqlx.Rows, error) {
	r, err := st.stmts[st.db.slave()].Queryx(args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			r, err := st.stmts[st.db.slave()].Queryx(args...)
			if err == nil {
				return r, err
			}
		}
	}
	return r, err
}

//QueryRowx will always go to slave
func (st *Stmtx) QueryRowx(args ...interface{}) *sqlx.Row {
	return st.stmts[st.db.slave()].QueryRowx(args...)
}

//Get will always go to slave
func (st *Stmtx) Get(dest interface{}, args ...interface{}) error {
	return st.stmts[st.db.slave()].Get(dest, args...)
}

//GetMaster will always go to master
func (st *Stmtx) GetMaster(dest interface{}, args ...interface{}) error {
	return st.stmts[0].Get(dest, args...)
}

//PersistentGet will always go to slave persistently
func (st *Stmtx) PersistentGet(dest interface{}, args ...interface{}) error {
	err := st.stmts[st.db.slave()].Get(dest, args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			err := st.stmts[st.db.slave()].Get(dest, args...)
			if err == nil {
				return err
			}
		}
	}
	return err
}

//Select will always go to slave
func (st *Stmtx) Select(dest interface{}, args ...interface{}) error {
	return st.stmts[st.db.slave()].Select(dest, args...)
}

//SelectMaster will always go to master
func (st *Stmtx) SelectMaster(dest interface{}, args ...interface{}) error {
	return st.stmts[0].Select(dest, args...)
}

//PersistentSelect will always go to slave persistently
func (st *Stmtx) PersistentSelect(dest interface{}, args ...interface{}) error {
	err := st.stmts[st.db.slave()].Select(dest, args...)
	if err != nil {
		for i := 0; i < maxRetry; i++ {
			err := st.stmts[st.db.slave()].Select(dest, args...)
			if err == nil {
				return err
			}
		}
	}
	return err
}

//slave
func (db *DB) slave() int {
	if db.length <= 1 {
		return 0
	}

	slave := int(1 + (atomic.AddUint64(&db.count, 1) % uint64(db.length-1)))
	return db.activedb[slave]
}

//SetMaxRetry attempt for persistent functions, default : 1
func SetMaxRetry(max int) {
	maxRetry = max
}

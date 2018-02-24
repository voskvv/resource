package postgres

import (
	"context"
	"fmt"

	"strings"

	"git.containerum.net/ch/resource-service/models"
	chutils "git.containerum.net/ch/utils"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // postgresql database driver
	"github.com/mattes/migrate"
	migdrv "github.com/mattes/migrate/database/postgres"
	_ "github.com/mattes/migrate/source/file" // needed to load migrations scripts from files
	"github.com/sirupsen/logrus"

	rstypes "git.containerum.net/ch/json-types/resource-service"
)

type pgDB struct {
	conn     *sqlx.DB // do not use for operations
	extLog   sqlx.ExtContext
	log      *logrus.Entry
	preparer chutils.SQLXPreparer

	// for information
	pgConnStr          string
	migrations         string
	migrationsVerstion string
}

// DBConnect initializes connection to postgresql database.
// github.com/jmoiron/sqlx used to to get work with database.
// Function tries to ping database and apply migrations using github.com/mattes/migrate.
// If migrations applying failed database goes to dirty state and requires manual conflict resolution.
func DBConnect(pgConnStr string, migrations string) (models.DB, error) {
	log := logrus.WithField("component", "postgres_db")
	log.Infoln("Connecting to ", pgConnStr)
	conn, err := sqlx.Connect("postgres", pgConnStr)
	if err != nil {
		log.WithError(err).Errorln("postgres connection failed")
		return nil, err
	}

	ret := &pgDB{
		conn:     conn,
		log:      log,
		extLog:   chutils.NewSQLXExtContextLogger(conn, log),
		preparer: chutils.NewSQLXPreparerLogger(conn, log),
	}

	m, err := ret.migrateUp(migrations)
	if err != nil {
		return nil, err
	}
	version, dirty, err := m.Version()
	log.WithError(err).WithFields(logrus.Fields{
		"dirty":   dirty,
		"version": version,
	}).Infoln("Migrate up")

	ret.pgConnStr = pgConnStr
	ret.migrations = migrations
	ret.migrationsVerstion = fmt.Sprintf("%v; dirty = %v", version, dirty)

	return ret, nil
}

func (db *pgDB) migrateUp(path string) (*migrate.Migrate, error) {
	db.log.Infof("Running migrations")
	instance, err := migdrv.WithInstance(db.conn.DB, &migdrv.Config{})
	if err != nil {
		return nil, err
	}
	m, err := migrate.NewWithDatabaseInstance(path, "clickhouse", instance)
	if err != nil {
		return nil, err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return nil, err
	}
	return m, nil
}

func (db *pgDB) Transactional(ctx context.Context, f func(ctx context.Context, tx models.DB) error) (err error) {
	e := db.log.WithField("transaction_id", chutils.NewUUID())
	e.Debugln("Begin transaction")
	tx, txErr := db.conn.Beginx()
	if txErr != nil {
		e.WithError(txErr).Errorln("Begin transaction error")
		return models.ErrTransactionBegin
	}

	arg := &pgDB{
		conn:     db.conn,
		log:      e,
		extLog:   chutils.NewSQLXExtContextLogger(tx, e),
		preparer: chutils.NewSQLXPreparerLogger(tx, e),
	}

	// needed for recovering panics in transactions.
	defer func(dberr error) {
		// if panic recovered, try to rollback transaction
		if panicErr := recover(); panicErr != nil {
			dberr = fmt.Errorf("panic in transaction: %v", panicErr)
		}

		if dberr != nil {
			e.WithError(dberr).Debugln("Rollback transaction")
			if rerr := tx.Rollback(); rerr != nil {
				e.WithError(rerr).Errorln("Rollback error")
				err = models.ErrTransactionRollback
			}
			err = dberr // forward error with panic description
			return
		}

		e.Debugln("Commit transaction")
		if cerr := tx.Commit(); cerr != nil {
			e.WithError(cerr).Errorln("Commit error")
			err = models.ErrTransactionCommit
		}
	}(f(ctx, arg))

	return
}

func createValues(vals []string) (ret string) {
	for _, v := range vals {
		ret += "('" + v + "'), "
	}
	ret = strings.TrimSuffix(ret, ", ")
	return
}

func (db *pgDB) String() string {
	return fmt.Sprintf("address: %s, migrations path: %s (version: %s)",
		db.pgConnStr, db.migrations, db.migrationsVerstion)
}

func (db *pgDB) Close() error {
	return db.conn.Close()
}

func (db *pgDB) GetResourcesCount(ctx context.Context, userID string) (ret rstypes.GetResourcesCountResponse, err error) {
	db.log.WithField("user_id", userID).Debug("get resources count")

	var nsIDs []string
	query, args, _ := sqlx.Named( /* language=sql */
		`SELECT resource_id FROM permissions WHERE (user_id, kind) = (:user_id, 'namespace')`,
		map[string]interface{}{"user_id": userID})
	err = sqlx.SelectContext(ctx, db.extLog, &nsIDs, db.extLog.Rebind(query), args...)
	if err != nil {
		err = models.WrapDBError(err)
		return
	}

	ret.Namespaces = len(nsIDs)

	var volservs struct {
		Volumes     int `db:"volcnt"`
		ExtServices int `db:"extcnt"`
		IntServices int `db:"intcnt"`
	}
	query, args, _ = sqlx.Named( /* language=sql */
		`SELECT
			count(*) FILTER (WHERE kind = 'volume') AS volcnt,
			count(*) FILTER (WHERE kind = 'extservice') AS extcnt,
			count(*) FILTER (WHERE kind = 'intservice') AS intcnt
		FROM permissions
		WHERE user_id = :user_id`,
		map[string]interface{}{"user_id": userID})
	err = sqlx.GetContext(ctx, db.extLog, &volservs, db.extLog.Rebind(query), args...)
	if err != nil {
		err = models.WrapDBError(err)
		return
	}

	ret.Volumes = volservs.Volumes
	ret.ExtServices = volservs.ExtServices
	ret.IntServices = volservs.IntServices

	var deplIDs []string
	query, args, _ = sqlx.In( /* language=sql */ `SELECT * FROM deployments WHERE ns_id IN (?)`, nsIDs)
	err = sqlx.SelectContext(ctx, db.extLog, deplIDs, db.extLog.Rebind(query), args...)
	if err != nil {
		err = models.WrapDBError(err)
		return
	}

	ret.Deployments = len(deplIDs)

	query, args, _ = sqlx.In( /* language=sql */
		`SELECT count(*) 
		FROM ingresses i
		JOIN services s ON i.service_id = s.id
		WHERE s.deploy_id IN (?)`,
		deplIDs)
	err = sqlx.GetContext(ctx, db.extLog, &ret.Ingresses, db.extLog.Rebind(query), args...)
	if err != nil {
		err = models.WrapDBError(err)
		return
	}

	query, args, _ = sqlx.In( /* language=sql */
		`SELECT count(c.*)
		FROM containers
		WHERE depl_id IN (?)`,
		deplIDs)
	err = sqlx.GetContext(ctx, db.extLog, &ret.Containers, db.extLog.Rebind(query), args...)
	if err != nil {
		err = models.WrapDBError(err)
	}

	return
}

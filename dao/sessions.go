package dao

import (
	"context"
	"fmt"
	"strings"
	"time"

	"TUM-Live-Backend/model"

	"github.com/guregu/null"
	"github.com/satori/go.uuid"
)

var (
	_ = time.Second
	_ = null.Bool{}
	_ = uuid.UUID{}
)

// GetAllSessions is a function to get a slice of record(s) from sessions table in the rbglive database
// params - page     - page requested (defaults to 0)
// params - pagesize - number of records in a page  (defaults to 20)
// params - order    - db sort order column
// error - ErrNotFound, db Find error
func GetAllSessions(ctx context.Context, page, pagesize int64, order string) (results []*model.Sessions, totalRows int, err error) {
	sql := "SELECT * FROM `sessions`"

	if order != "" {
		if strings.ContainsAny(order, "'\"") {
			order = ""
		}
	}

	if order == "" {
		order = "id"
	}

	if DB.DriverName() == "mssql" {
		sql = fmt.Sprintf("%s order by %s OFFSET %d ROWS FETCH FIRST %d ROWS ONLY", sql, order, page, pagesize)
	} else if DB.DriverName() == "postgres" {
		sql = fmt.Sprintf("%s order by `%s` OFFSET %d LIMIT %d", sql, order, page, pagesize)
	} else {
		sql = fmt.Sprintf("%s order by `%s` LIMIT %d, %d", sql, order, page, pagesize)
	}
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	err = DB.SelectContext(ctx, &results, sql)
	if err != nil {
		return nil, -1, err
	}

	cnt, err := GetRowCount(ctx, "sessions")
	if err != nil {
		return results, -2, err
	}

	return results, cnt, err
}

// GetSessions is a function to get a single record from the sessions table in the rbglive database
// error - ErrNotFound, db Find error
func GetSessions(ctx context.Context, argID int32) (record *model.Sessions, err error) {
	sql := "SELECT * FROM `sessions` WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	record = &model.Sessions{}
	err = DB.GetContext(ctx, record, sql, argID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// AddSessions is a function to add a single record to sessions table in the rbglive database
// error - ErrInsertFailed, db save call failed
func AddSessions(ctx context.Context, record *model.Sessions) (result *model.Sessions, RowsAffected int64, err error) {
	if DB.DriverName() == "postgres" {
		return addSessionsPostgres(ctx, record)
	} else {
		return addSessions(ctx, record)
	}
}

// addSessionsPostgres is a function to add a single record to sessions table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addSessionsPostgres(ctx context.Context, record *model.Sessions) (result *model.Sessions, RowsAffected int64, err error) {
	sql := "INSERT INTO `sessions` ( id,  created,  sessionId,  userId) values ( ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(1)
	sql = fmt.Sprintf("%s returning %s", sql, "id")
	dbResult := DB.QueryRowContext(ctx, sql, record.ID, record.Created, record.SessionID, record.UserID)
	err = dbResult.Scan(record.ID, record.Created, record.SessionID, record.UserID)

	return record, rows, err
}

// addSessionsPostgres is a function to add a single record to sessions table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addSessions(ctx context.Context, record *model.Sessions) (result *model.Sessions, RowsAffected int64, err error) {
	sql := "INSERT INTO `sessions` ( id,  created,  sessionId,  userId) values ( ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(0)

	dbResult, err := DB.ExecContext(ctx, sql, record.ID, record.Created, record.SessionID, record.UserID)
	if err != nil {
		return nil, 0, err
	}

	id, err := dbResult.LastInsertId()
	rows, err = dbResult.RowsAffected()

	record.ID = int32(id)

	return record, rows, err
}

// UpdateSessions is a function to update a single record from sessions table in the rbglive database
// error - ErrNotFound, db record for id not found
// error - ErrUpdateFailed, db meta data copy failed or db.Save call failed
func UpdateSessions(ctx context.Context, argID int32, updated *model.Sessions) (result *model.Sessions, RowsAffected int64, err error) {
	sql := "UPDATE `sessions` set created = ?, sessionId = ?, userId = ? WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	dbResult, err := DB.ExecContext(ctx, sql, updated.Created, updated.SessionID, updated.UserID, argID)
	if err != nil {
		return nil, 0, err
	}

	rows, err := dbResult.RowsAffected()
	updated.ID = argID

	return updated, rows, err
}

// DeleteSessions is a function to delete a single record from sessions table in the rbglive database
// error - ErrNotFound, db Find error
// error - ErrDeleteFailed, db Delete failed error
func DeleteSessions(ctx context.Context, argID int32) (rowsAffected int64, err error) {
	sql := "DELETE FROM `sessions` where id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	result, err := DB.ExecContext(ctx, sql, argID)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

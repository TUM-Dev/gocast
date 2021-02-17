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

// GetAllStreams is a function to get a slice of record(s) from streams table in the rbglive database
// params - page     - page requested (defaults to 0)
// params - pagesize - number of records in a page  (defaults to 20)
// params - order    - db sort order column
// error - ErrNotFound, db Find error
func GetAllStreams(ctx context.Context, page, pagesize int64, order string) (results []*model.Streams, totalRows int, err error) {
	sql := "SELECT * FROM `streams`"

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

	cnt, err := GetRowCount(ctx, "streams")
	if err != nil {
		return results, -2, err
	}

	return results, cnt, err
}

// GetStreams is a function to get a single record from the streams table in the rbglive database
// error - ErrNotFound, db Find error
func GetStreams(ctx context.Context, argID int32) (record *model.Streams, err error) {
	sql := "SELECT * FROM `streams` WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	record = &model.Streams{}
	err = DB.GetContext(ctx, record, sql, argID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// AddStreams is a function to add a single record to streams table in the rbglive database
// error - ErrInsertFailed, db save call failed
func AddStreams(ctx context.Context, record *model.Streams) (result *model.Streams, RowsAffected int64, err error) {
	if DB.DriverName() == "postgres" {
		return addStreamsPostgres(ctx, record)
	} else {
		return addStreams(ctx, record)
	}
}

// addStreamsPostgres is a function to add a single record to streams table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addStreamsPostgres(ctx context.Context, record *model.Streams) (result *model.Streams, RowsAffected int64, err error) {
	sql := "INSERT INTO `streams` ( id,  start,  end,  streamkey,  courseId,  vodEnabled) values ( ?, ?, ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(1)
	sql = fmt.Sprintf("%s returning %s", sql, "id")
	dbResult := DB.QueryRowContext(ctx, sql, record.ID, record.Start, record.End, record.Streamkey, record.CourseID, record.VodEnabled)
	err = dbResult.Scan(record.ID, record.Start, record.End, record.Streamkey, record.CourseID, record.VodEnabled)

	return record, rows, err
}

// addStreamsPostgres is a function to add a single record to streams table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addStreams(ctx context.Context, record *model.Streams) (result *model.Streams, RowsAffected int64, err error) {
	sql := "INSERT INTO `streams` ( id,  start,  end,  streamkey,  courseId,  vodEnabled) values ( ?, ?, ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(0)

	dbResult, err := DB.ExecContext(ctx, sql, record.ID, record.Start, record.End, record.Streamkey, record.CourseID, record.VodEnabled)
	if err != nil {
		return nil, 0, err
	}

	id, err := dbResult.LastInsertId()
	rows, err = dbResult.RowsAffected()

	record.ID = int32(id)

	return record, rows, err
}

// UpdateStreams is a function to update a single record from streams table in the rbglive database
// error - ErrNotFound, db record for id not found
// error - ErrUpdateFailed, db meta data copy failed or db.Save call failed
func UpdateStreams(ctx context.Context, argID int32, updated *model.Streams) (result *model.Streams, RowsAffected int64, err error) {
	sql := "UPDATE `streams` set start = ?, end = ?, streamkey = ?, courseId = ?, vodEnabled = ? WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	dbResult, err := DB.ExecContext(ctx, sql, updated.Start, updated.End, updated.Streamkey, updated.CourseID, updated.VodEnabled, argID)
	if err != nil {
		return nil, 0, err
	}

	rows, err := dbResult.RowsAffected()
	updated.ID = argID

	return updated, rows, err
}

// DeleteStreams is a function to delete a single record from streams table in the rbglive database
// error - ErrNotFound, db Find error
// error - ErrDeleteFailed, db Delete failed error
func DeleteStreams(ctx context.Context, argID int32) (rowsAffected int64, err error) {
	sql := "DELETE FROM `streams` where id = ?"
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

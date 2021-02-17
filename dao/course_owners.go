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

// GetAllCourseOwners is a function to get a slice of record(s) from course_owners table in the rbglive database
// params - page     - page requested (defaults to 0)
// params - pagesize - number of records in a page  (defaults to 20)
// params - order    - db sort order column
// error - ErrNotFound, db Find error
func GetAllCourseOwners(ctx context.Context, page, pagesize int64, order string) (results []*model.CourseOwners, totalRows int, err error) {
	sql := "SELECT * FROM `course_owners`"

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

	cnt, err := GetRowCount(ctx, "course_owners")
	if err != nil {
		return results, -2, err
	}

	return results, cnt, err
}

// GetCourseOwners is a function to get a single record from the course_owners table in the rbglive database
// error - ErrNotFound, db Find error
func GetCourseOwners(ctx context.Context, argID int32) (record *model.CourseOwners, err error) {
	sql := "SELECT * FROM `course_owners` WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	record = &model.CourseOwners{}
	err = DB.GetContext(ctx, record, sql, argID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// AddCourseOwners is a function to add a single record to course_owners table in the rbglive database
// error - ErrInsertFailed, db save call failed
func AddCourseOwners(ctx context.Context, record *model.CourseOwners) (result *model.CourseOwners, RowsAffected int64, err error) {
	if DB.DriverName() == "postgres" {
		return addCourseOwnersPostgres(ctx, record)
	} else {
		return addCourseOwners(ctx, record)
	}
}

// addCourseOwnersPostgres is a function to add a single record to course_owners table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addCourseOwnersPostgres(ctx context.Context, record *model.CourseOwners) (result *model.CourseOwners, RowsAffected int64, err error) {
	sql := "INSERT INTO `course_owners` ( id,  userid,  courseid) values ( ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(1)
	sql = fmt.Sprintf("%s returning %s", sql, "id")
	dbResult := DB.QueryRowContext(ctx, sql, record.ID, record.Userid, record.Courseid)
	err = dbResult.Scan(record.ID, record.Userid, record.Courseid)

	return record, rows, err
}

// addCourseOwnersPostgres is a function to add a single record to course_owners table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addCourseOwners(ctx context.Context, record *model.CourseOwners) (result *model.CourseOwners, RowsAffected int64, err error) {
	sql := "INSERT INTO `course_owners` ( id,  userid,  courseid) values ( ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(0)

	dbResult, err := DB.ExecContext(ctx, sql, record.ID, record.Userid, record.Courseid)
	if err != nil {
		return nil, 0, err
	}

	id, err := dbResult.LastInsertId()
	rows, err = dbResult.RowsAffected()

	record.ID = int32(id)

	return record, rows, err
}

// UpdateCourseOwners is a function to update a single record from course_owners table in the rbglive database
// error - ErrNotFound, db record for id not found
// error - ErrUpdateFailed, db meta data copy failed or db.Save call failed
func UpdateCourseOwners(ctx context.Context, argID int32, updated *model.CourseOwners) (result *model.CourseOwners, RowsAffected int64, err error) {
	sql := "UPDATE `course_owners` set userid = ?, courseid = ? WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	dbResult, err := DB.ExecContext(ctx, sql, updated.Userid, updated.Courseid, argID)
	if err != nil {
		return nil, 0, err
	}

	rows, err := dbResult.RowsAffected()
	updated.ID = argID

	return updated, rows, err
}

// DeleteCourseOwners is a function to delete a single record from course_owners table in the rbglive database
// error - ErrNotFound, db Find error
// error - ErrDeleteFailed, db Delete failed error
func DeleteCourseOwners(ctx context.Context, argID int32) (rowsAffected int64, err error) {
	sql := "DELETE FROM `course_owners` where id = ?"
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

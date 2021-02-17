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

// GetAllCourses is a function to get a slice of record(s) from courses table in the rbglive database
// params - page     - page requested (defaults to 0)
// params - pagesize - number of records in a page  (defaults to 20)
// params - order    - db sort order column
// error - ErrNotFound, db Find error
func GetAllCourses(ctx context.Context, page, pagesize int64, order string) (results []*model.Courses, totalRows int, err error) {
	sql := "SELECT * FROM `courses`"

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

	cnt, err := GetRowCount(ctx, "courses")
	if err != nil {
		return results, -2, err
	}

	return results, cnt, err
}

// GetCourses is a function to get a single record from the courses table in the rbglive database
// error - ErrNotFound, db Find error
func GetCourses(ctx context.Context, argID int32) (record *model.Courses, err error) {
	sql := "SELECT * FROM `courses` WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	record = &model.Courses{}
	err = DB.GetContext(ctx, record, sql, argID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// AddCourses is a function to add a single record to courses table in the rbglive database
// error - ErrInsertFailed, db save call failed
func AddCourses(ctx context.Context, record *model.Courses) (result *model.Courses, RowsAffected int64, err error) {
	if DB.DriverName() == "postgres" {
		return addCoursesPostgres(ctx, record)
	} else {
		return addCourses(ctx, record)
	}
}

// addCoursesPostgres is a function to add a single record to courses table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addCoursesPostgres(ctx context.Context, record *model.Courses) (result *model.Courses, RowsAffected int64, err error) {
	sql := "INSERT INTO `courses` ( id,  name,  start,  end,  semester) values ( ?, ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(1)
	sql = fmt.Sprintf("%s returning %s", sql, "id")
	dbResult := DB.QueryRowContext(ctx, sql, record.ID, record.Name, record.Start, record.End, record.Semester)
	err = dbResult.Scan(record.ID, record.Name, record.Start, record.End, record.Semester)

	return record, rows, err
}

// addCoursesPostgres is a function to add a single record to courses table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addCourses(ctx context.Context, record *model.Courses) (result *model.Courses, RowsAffected int64, err error) {
	sql := "INSERT INTO `courses` ( id,  name,  start,  end,  semester) values ( ?, ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(0)

	dbResult, err := DB.ExecContext(ctx, sql, record.ID, record.Name, record.Start, record.End, record.Semester)
	if err != nil {
		return nil, 0, err
	}

	id, err := dbResult.LastInsertId()
	rows, err = dbResult.RowsAffected()

	record.ID = int32(id)

	return record, rows, err
}

// UpdateCourses is a function to update a single record from courses table in the rbglive database
// error - ErrNotFound, db record for id not found
// error - ErrUpdateFailed, db meta data copy failed or db.Save call failed
func UpdateCourses(ctx context.Context, argID int32, updated *model.Courses) (result *model.Courses, RowsAffected int64, err error) {
	sql := "UPDATE `courses` set name = ?, start = ?, end = ?, semester = ? WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	dbResult, err := DB.ExecContext(ctx, sql, updated.Name, updated.Start, updated.End, updated.Semester, argID)
	if err != nil {
		return nil, 0, err
	}

	rows, err := dbResult.RowsAffected()
	updated.ID = argID

	return updated, rows, err
}

// DeleteCourses is a function to delete a single record from courses table in the rbglive database
// error - ErrNotFound, db Find error
// error - ErrDeleteFailed, db Delete failed error
func DeleteCourses(ctx context.Context, argID int32) (rowsAffected int64, err error) {
	sql := "DELETE FROM `courses` where id = ?"
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

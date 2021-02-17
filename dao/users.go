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

// GetAllUsers is a function to get a slice of record(s) from users table in the rbglive database
// params - page     - page requested (defaults to 0)
// params - pagesize - number of records in a page  (defaults to 20)
// params - order    - db sort order column
// error - ErrNotFound, db Find error
func GetAllUsers(ctx context.Context, page, pagesize int64, order string) (results []*model.Users, totalRows int, err error) {
	sql := "SELECT * FROM `users`"

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

	cnt, err := GetRowCount(ctx, "users")
	if err != nil {
		return results, -2, err
	}

	return results, cnt, err
}

// GetUsers is a function to get a single record from the users table in the rbglive database
// error - ErrNotFound, db Find error
func GetUsers(ctx context.Context, argID int32) (record *model.Users, err error) {
	sql := "SELECT * FROM `users` WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	record = &model.Users{}
	err = DB.GetContext(ctx, record, sql, argID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// AddUsers is a function to add a single record to users table in the rbglive database
// error - ErrInsertFailed, db save call failed
func AddUsers(ctx context.Context, record *model.Users) (result *model.Users, RowsAffected int64, err error) {
	if DB.DriverName() == "postgres" {
		return addUsersPostgres(ctx, record)
	} else {
		return addUsers(ctx, record)
	}
}

// addUsersPostgres is a function to add a single record to users table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addUsersPostgres(ctx context.Context, record *model.Users) (result *model.Users, RowsAffected int64, err error) {
	sql := "INSERT INTO `users` ( id,  name,  email,  role,  password) values ( ?, ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(1)
	sql = fmt.Sprintf("%s returning %s", sql, "id")
	dbResult := DB.QueryRowContext(ctx, sql, record.ID, record.Name, record.Email, record.Role, record.Password)
	err = dbResult.Scan(record.ID, record.Name, record.Email, record.Role, record.Password)

	return record, rows, err
}

// addUsersPostgres is a function to add a single record to users table in the rbglive database
// error - ErrInsertFailed, db save call failed
func addUsers(ctx context.Context, record *model.Users) (result *model.Users, RowsAffected int64, err error) {
	sql := "INSERT INTO `users` ( id,  name,  email,  role,  password) values ( ?, ?, ?, ?, ? )"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	rows := int64(0)

	dbResult, err := DB.ExecContext(ctx, sql, record.ID, record.Name, record.Email, record.Role, record.Password)
	if err != nil {
		return nil, 0, err
	}

	id, err := dbResult.LastInsertId()
	rows, err = dbResult.RowsAffected()

	record.ID = int32(id)

	return record, rows, err
}

// UpdateUser is a function to update a single record from users table in the rbglive database
// error - ErrNotFound, db record for id not found
// error - ErrUpdateFailed, db meta data copy failed or db.Save call failed
func UpdateUser(ctx context.Context, argID int32, updated *model.Users) (result *model.Users, RowsAffected int64, err error) {
	sql := "UPDATE `users` set name = ?, email = ?, role = ?, password = ? WHERE id = ?"
	sql = DB.Rebind(sql)

	if Logger != nil {
		Logger(ctx, sql)
	}

	dbResult, err := DB.ExecContext(ctx, sql, updated.Name, updated.Email, updated.Role, updated.Password, argID)
	if err != nil {
		return nil, 0, err
	}

	rows, err := dbResult.RowsAffected()
	updated.ID = argID

	return updated, rows, err
}

// DeleteUser is a function to delete a single record from users table in the rbglive database
// error - ErrNotFound, db Find error
// error - ErrDeleteFailed, db Delete failed error
func DeleteUser(ctx context.Context, argID int32) (rowsAffected int64, err error) {
	sql := "DELETE FROM `users` where id = ?"
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

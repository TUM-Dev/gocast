package model

import (
	"database/sql"
	"time"

	"github.com/guregu/null"
	"github.com/satori/go.uuid"
)

var (
	_ = time.Second
	_ = sql.LevelDefault
	_ = null.Bool{}
	_ = uuid.UUID{}
)

/*
DB Table Details
-------------------------------------


Table: courses
[ 0] id                                             INT4                 null: false  primary: true   isArray: false  auto: false  col: INT4            len: -1      default: []
[ 1] name                                           TEXT                 null: false  primary: false  isArray: false  auto: false  col: TEXT            len: -1      default: []
[ 2] start                                          DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
[ 3] end                                            DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
[ 4] semester                                       TEXT                 null: true   primary: false  isArray: false  auto: false  col: TEXT            len: -1      default: []


JSON Sample
-------------------------------------
{    "Id": 44,    "Name": "eLtHjUNZxaXhuedVENIQYdaxw",    "Start": "2165-08-09T01:05:44.329163825+01:00",    "End": "2290-11-16T04:16:47.5391697+01:00",    "Semester": "jXUyKeprFSkvCZqCeDAERBTbY"}



*/

// Courses struct is a row record of the courses table in the rbglive database
type Courses struct {
	//[ 0] id                                             INT4                 null: false  primary: true   isArray: false  auto: false  col: INT4            len: -1      default: []
	ID int32
	//[ 1] name                                           TEXT                 null: false  primary: false  isArray: false  auto: false  col: TEXT            len: -1      default: []
	Name string
	//[ 2] start                                          DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
	Start time.Time
	//[ 3] end                                            DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
	End time.Time
	//[ 4] semester                                       TEXT                 null: true   primary: false  isArray: false  auto: false  col: TEXT            len: -1      default: []
	Semester sql.NullString
}

var coursesTableInfo = &TableInfo{
	Name: "courses",
	Columns: []*ColumnInfo{

		&ColumnInfo{
			Index:              0,
			Name:               "id",
			Comment:            ``,
			Notes:              ``,
			Nullable:           false,
			DatabaseTypeName:   "INT4",
			DatabaseTypePretty: "INT4",
			IsPrimaryKey:       true,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "INT4",
			ColumnLength:       -1,
			GoFieldName:        "ID",
			GoFieldType:        "int32",
			JSONFieldName:      "Id",
			ProtobufFieldName:  "id",
			ProtobufType:       "int32",
			ProtobufPos:        1,
		},

		&ColumnInfo{
			Index:              1,
			Name:               "name",
			Comment:            ``,
			Notes:              ``,
			Nullable:           false,
			DatabaseTypeName:   "TEXT",
			DatabaseTypePretty: "TEXT",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "TEXT",
			ColumnLength:       -1,
			GoFieldName:        "Name",
			GoFieldType:        "string",
			JSONFieldName:      "Name",
			ProtobufFieldName:  "name",
			ProtobufType:       "string",
			ProtobufPos:        2,
		},

		&ColumnInfo{
			Index:              2,
			Name:               "start",
			Comment:            ``,
			Notes:              ``,
			Nullable:           true,
			DatabaseTypeName:   "DATE",
			DatabaseTypePretty: "DATE",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "DATE",
			ColumnLength:       -1,
			GoFieldName:        "Start",
			GoFieldType:        "time.Time",
			JSONFieldName:      "Start",
			ProtobufFieldName:  "start",
			ProtobufType:       "google.protobuf.Timestamp",
			ProtobufPos:        3,
		},

		&ColumnInfo{
			Index:              3,
			Name:               "end",
			Comment:            ``,
			Notes:              ``,
			Nullable:           true,
			DatabaseTypeName:   "DATE",
			DatabaseTypePretty: "DATE",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "DATE",
			ColumnLength:       -1,
			GoFieldName:        "End",
			GoFieldType:        "time.Time",
			JSONFieldName:      "End",
			ProtobufFieldName:  "end",
			ProtobufType:       "google.protobuf.Timestamp",
			ProtobufPos:        4,
		},

		&ColumnInfo{
			Index:              4,
			Name:               "semester",
			Comment:            ``,
			Notes:              ``,
			Nullable:           true,
			DatabaseTypeName:   "TEXT",
			DatabaseTypePretty: "TEXT",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "TEXT",
			ColumnLength:       -1,
			GoFieldName:        "Semester",
			GoFieldType:        "sql.NullString",
			JSONFieldName:      "Semester",
			ProtobufFieldName:  "semester",
			ProtobufType:       "string",
			ProtobufPos:        5,
		},
	},
}

// TableName sets the insert table name for this struct type
func (c *Courses) TableName() string {
	return "courses"
}

// BeforeSave invoked before saving, return an error if field is not populated.
func (c *Courses) BeforeSave() error {
	return nil
}

// Prepare invoked before saving, can be used to populate fields etc.
func (c *Courses) Prepare() {
}

// Validate invoked before performing action, return an error if field is not populated.
func (c *Courses) Validate(action Action) error {
	return nil
}

// TableInfo return table meta data
func (c *Courses) TableInfo() *TableInfo {
	return coursesTableInfo
}

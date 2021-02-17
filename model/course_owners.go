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


Table: course_owners
[ 0] id                                             INT4                 null: false  primary: true   isArray: false  auto: false  col: INT4            len: -1      default: []
[ 1] userid                                         INT4                 null: true   primary: false  isArray: false  auto: false  col: INT4            len: -1      default: []
[ 2] courseid                                       INT4                 null: true   primary: false  isArray: false  auto: false  col: INT4            len: -1      default: []


JSON Sample
-------------------------------------
{    "Id": 68,    "Userid": 1,    "Courseid": 55}



*/

// CourseOwners struct is a row record of the course_owners table in the rbglive database
type CourseOwners struct {
	//[ 0] id                                             INT4                 null: false  primary: true   isArray: false  auto: false  col: INT4            len: -1      default: []
	ID int32
	//[ 1] userid                                         INT4                 null: true   primary: false  isArray: false  auto: false  col: INT4            len: -1      default: []
	Userid sql.NullInt64
	//[ 2] courseid                                       INT4                 null: true   primary: false  isArray: false  auto: false  col: INT4            len: -1      default: []
	Courseid sql.NullInt64
}

var course_ownersTableInfo = &TableInfo{
	Name: "course_owners",
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
			Name:               "userid",
			Comment:            ``,
			Notes:              ``,
			Nullable:           true,
			DatabaseTypeName:   "INT4",
			DatabaseTypePretty: "INT4",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "INT4",
			ColumnLength:       -1,
			GoFieldName:        "Userid",
			GoFieldType:        "sql.NullInt64",
			JSONFieldName:      "Userid",
			ProtobufFieldName:  "userid",
			ProtobufType:       "int32",
			ProtobufPos:        2,
		},

		&ColumnInfo{
			Index:              2,
			Name:               "courseid",
			Comment:            ``,
			Notes:              ``,
			Nullable:           true,
			DatabaseTypeName:   "INT4",
			DatabaseTypePretty: "INT4",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "INT4",
			ColumnLength:       -1,
			GoFieldName:        "Courseid",
			GoFieldType:        "sql.NullInt64",
			JSONFieldName:      "Courseid",
			ProtobufFieldName:  "courseid",
			ProtobufType:       "int32",
			ProtobufPos:        3,
		},
	},
}

// TableName sets the insert table name for this struct type
func (c *CourseOwners) TableName() string {
	return "course_owners"
}

// BeforeSave invoked before saving, return an error if field is not populated.
func (c *CourseOwners) BeforeSave() error {
	return nil
}

// Prepare invoked before saving, can be used to populate fields etc.
func (c *CourseOwners) Prepare() {
}

// Validate invoked before performing action, return an error if field is not populated.
func (c *CourseOwners) Validate(action Action) error {
	return nil
}

// TableInfo return table meta data
func (c *CourseOwners) TableInfo() *TableInfo {
	return course_ownersTableInfo
}

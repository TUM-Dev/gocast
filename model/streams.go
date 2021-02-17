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


Table: streams
[ 0] id                                             INT4                 null: false  primary: true   isArray: false  auto: false  col: INT4            len: -1      default: []
[ 1] start                                          DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
[ 2] end                                            DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
[ 3] streamkey                                      TEXT                 null: true   primary: false  isArray: false  auto: false  col: TEXT            len: -1      default: []
[ 4] courseId                                       INT4                 null: true   primary: false  isArray: false  auto: false  col: INT4            len: -1      default: []
[ 5] vodEnabled                                     BOOL                 null: true   primary: false  isArray: false  auto: false  col: BOOL            len: -1      default: [true]


JSON Sample
-------------------------------------
{    "VodEnabled": false,    "Id": 93,    "Start": "2261-09-17T03:50:49.948545483+01:00",    "End": "2194-05-27T15:35:51.008796589+01:00",    "Streamkey": "GckbWqPJopOtNvaKrAAHkiESA",    "CourseId": 48}



*/

// Streams struct is a row record of the streams table in the rbglive database
type Streams struct {
	//[ 0] id                                             INT4                 null: false  primary: true   isArray: false  auto: false  col: INT4            len: -1      default: []
	ID int32
	//[ 1] start                                          DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
	Start time.Time
	//[ 2] end                                            DATE                 null: true   primary: false  isArray: false  auto: false  col: DATE            len: -1      default: []
	End time.Time
	//[ 3] streamkey                                      TEXT                 null: true   primary: false  isArray: false  auto: false  col: TEXT            len: -1      default: []
	Streamkey sql.NullString
	//[ 4] courseId                                       INT4                 null: true   primary: false  isArray: false  auto: false  col: INT4            len: -1      default: []
	CourseID sql.NullInt64
	//[ 5] vodEnabled                                     BOOL                 null: true   primary: false  isArray: false  auto: false  col: BOOL            len: -1      default: [true]
	VodEnabled sql.NullBool
}

var streamsTableInfo = &TableInfo{
	Name: "streams",
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
			ProtobufPos:        2,
		},

		&ColumnInfo{
			Index:              2,
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
			ProtobufPos:        3,
		},

		&ColumnInfo{
			Index:              3,
			Name:               "streamkey",
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
			GoFieldName:        "Streamkey",
			GoFieldType:        "sql.NullString",
			JSONFieldName:      "Streamkey",
			ProtobufFieldName:  "streamkey",
			ProtobufType:       "string",
			ProtobufPos:        4,
		},

		&ColumnInfo{
			Index:              4,
			Name:               "courseId",
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
			GoFieldName:        "CourseID",
			GoFieldType:        "sql.NullInt64",
			JSONFieldName:      "CourseId",
			ProtobufFieldName:  "course_id",
			ProtobufType:       "int32",
			ProtobufPos:        5,
		},

		&ColumnInfo{
			Index:              5,
			Name:               "vodEnabled",
			Comment:            ``,
			Notes:              ``,
			Nullable:           true,
			DatabaseTypeName:   "BOOL",
			DatabaseTypePretty: "BOOL",
			IsPrimaryKey:       false,
			IsAutoIncrement:    false,
			IsArray:            false,
			ColumnType:         "BOOL",
			ColumnLength:       -1,
			GoFieldName:        "VodEnabled",
			GoFieldType:        "sql.NullBool",
			JSONFieldName:      "VodEnabled",
			ProtobufFieldName:  "vod_enabled",
			ProtobufType:       "bool",
			ProtobufPos:        6,
		},
	},
}

// TableName sets the insert table name for this struct type
func (s *Streams) TableName() string {
	return "streams"
}

// BeforeSave invoked before saving, return an error if field is not populated.
func (s *Streams) BeforeSave() error {
	return nil
}

// Prepare invoked before saving, can be used to populate fields etc.
func (s *Streams) Prepare() {
}

// Validate invoked before performing action, return an error if field is not populated.
func (s *Streams) Validate(action Action) error {
	return nil
}

// TableInfo return table meta data
func (s *Streams) TableInfo() *TableInfo {
	return streamsTableInfo
}

package model

import "fmt"

// Action CRUD actions
type Action int32

var (
	// Create action when record is created
	Create = Action(0)

	// RetrieveOne action when a record is retrieved from db
	RetrieveOne = Action(1)

	// RetrieveMany action when record(s) are retrieved from db
	RetrieveMany = Action(2)

	// Update action when record is updated in db
	Update = Action(3)

	// Delete action when record is deleted in db
	Delete = Action(4)

	// FetchDDL action when fetching ddl info from db
	FetchDDL = Action(5)

	tables map[string]*TableInfo
)

func init() {
	tables = make(map[string]*TableInfo)

	tables["course_owners"] = course_ownersTableInfo
	tables["courses"] = coursesTableInfo
	tables["sessions"] = sessionsTableInfo
	tables["streams"] = streamsTableInfo
	tables["users"] = usersTableInfo
}

// String describe the action
func (i Action) String() string {
	switch i {
	case Create:
		return "Create"
	case RetrieveOne:
		return "RetrieveOne"
	case RetrieveMany:
		return "RetrieveMany"
	case Update:
		return "Update"
	case Delete:
		return "Delete"
	case FetchDDL:
		return "FetchDDL"
	default:
		return fmt.Sprintf("unknown action: %d", int(i))
	}
}

// Model interface methods for database structs generated
type Model interface {
	TableName() string
	BeforeSave() error
	Prepare()
	Validate(action Action) error
	TableInfo() *TableInfo
}

// TableInfo describes a table in the database
type TableInfo struct {
	Name    string        `json:"Name"`
	Columns []*ColumnInfo `json:"Columns"`
}

// ColumnInfo describes a column in the database table
type ColumnInfo struct {
	Index              int    `json:"Index"`
	GoFieldName        string `json:"GoFieldName"`
	GoFieldType        string `json:"GoFieldType"`
	JSONFieldName      string `json:"JsonFieldName"`
	ProtobufFieldName  string `json:"ProtobufFieldName"`
	ProtobufType       string `json:"ProtobufFieldType"`
	ProtobufPos        int    `json:"ProtobufFieldPos"`
	Comment            string `json:"Comment"`
	Notes              string `json:"Notes"`
	Name               string `json:"Name"`
	Nullable           bool   `json:"IsNullable"`
	DatabaseTypeName   string `json:"DatabaseTypeName"`
	DatabaseTypePretty string `json:"DatabaseTypePretty"`
	IsPrimaryKey       bool   `json:"IsPrimaryKey"`
	IsAutoIncrement    bool   `json:"IsAutoIncrement"`
	IsArray            bool   `json:"IsArray"`
	ColumnType         string `json:"ColumnType"`
	ColumnLength       int64  `json:"ColumnLength"`
	DefaultValue       string `json:"DefaultValue"`
}

// GetTableInfo retrieve TableInfo for a table
func GetTableInfo(name string) (*TableInfo, bool) {
	val, ok := tables[name]
	return val, ok
}

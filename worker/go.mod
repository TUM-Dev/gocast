module github.com/TUM-Dev/gocast/worker

go 1.26.0

// Direct dependencies
require (
	github.com/icza/gox v0.2.0
	github.com/iris-contrib/go.uuid v2.0.0+incompatible
	github.com/joschahenningsen/thumbgen v0.1.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/sirupsen/logrus v1.9.3
	github.com/tidwall/gjson v1.18.0
	golang.org/x/sync v0.20.0
	google.golang.org/grpc v1.83.1
	google.golang.org/protobuf v1.36.11
)

require (
	github.com/pkg/profile v1.7.0
	github.com/shirou/gopsutil/v4 v4.25.9
)

// Indirect dependencies
require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20231016141302-07b5767bb0ed // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/u2takey/go-utils v0.3.1
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/google/pprof v0.0.0-20241210010833-40e02aabc2ad // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	golang.org/x/image v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
)

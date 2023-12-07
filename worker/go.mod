module github.com/TUM-Dev/gocast/worker

go 1.21.3

// Direct dependencies
require (
	github.com/getsentry/sentry-go v0.23.0
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/icza/gox v0.0.0-20230330130131-23e1aaac139e
	github.com/iris-contrib/go.uuid v2.0.0+incompatible
	github.com/joschahenningsen/thumbgen v0.1.2
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.23.7
	github.com/sirupsen/logrus v1.9.3
	github.com/tidwall/gjson v1.16.0
	golang.org/x/sync v0.3.0
	google.golang.org/grpc v1.57.0
	google.golang.org/protobuf v1.31.0
)

require (
	github.com/makasim/sentryhook v0.4.2
	github.com/pkg/profile v1.7.0
)

// Indirect dependencies
require (
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20230326075908-cb1d2100619a // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/u2takey/go-utils v0.3.1
	github.com/yusufpapurcu/wmi v1.2.3 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
)

require (
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/google/pprof v0.0.0-20230811205829-9131a7e9cc17 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	golang.org/x/image v0.11.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230814215434-ca7cfce7776a // indirect
)

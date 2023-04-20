module github.com/joschahenningsen/TUM-Live/worker

go 1.19

// Direct dependencies
require (
	github.com/getsentry/sentry-go v0.20.0
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/icza/gox v0.0.0-20230330130131-23e1aaac139e
	github.com/iris-contrib/go.uuid v2.0.0+incompatible
	github.com/joschahenningsen/thumbgen v0.1.0
	github.com/otiai10/gosseract/v2 v2.4.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.23.3
	github.com/sirupsen/logrus v1.9.0
	github.com/tidwall/gjson v1.14.4
	github.com/u2takey/ffmpeg-go v0.4.1
	golang.org/x/sync v0.1.0
	google.golang.org/grpc v1.54.0
	google.golang.org/protobuf v1.30.0
)

require (
	github.com/bbalet/stopwords v1.0.0
	github.com/makasim/sentryhook v0.4.2
	github.com/pkg/profile v1.7.0
)

// Indirect dependencies
require (
	github.com/aws/aws-sdk-go v1.44.246 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20230326075908-cb1d2100619a // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/u2takey/go-utils v0.3.1
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230410155749-daa745c078e1 // indirect
)

require (
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/google/pprof v0.0.0-20230406165453-00490a63f317 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.5 // indirect
)

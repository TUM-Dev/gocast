module github.com/joschahenningsen/TUM-Live/worker

go 1.19

// Direct dependencies
require (
	github.com/getsentry/sentry-go v0.15.0
	github.com/golang/protobuf v1.5.2
	github.com/icza/gox v0.0.0-20221026131554-a08a8cdc726a
	github.com/iris-contrib/go.uuid v2.0.0+incompatible
	github.com/joschahenningsen/thumbgen v0.0.0-20220618164424-9fcc2beb0084
	github.com/otiai10/gosseract/v2 v2.4.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.22.12
	github.com/sirupsen/logrus v1.9.0
	github.com/tidwall/gjson v1.14.4
	github.com/u2takey/ffmpeg-go v0.4.1
	golang.org/x/sync v0.1.0
	google.golang.org/grpc v1.51.0
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/bbalet/stopwords v1.0.0
	github.com/makasim/sentryhook v0.4.0
	github.com/pkg/profile v1.7.0
)

// Indirect dependencies
require (
	github.com/aws/aws-sdk-go v1.44.151 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/u2takey/go-utils v0.3.1
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/net v0.2.0 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/text v0.4.0 // indirect
	google.golang.org/genproto v0.0.0-20221201204527-e3fa12d562f3 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/google/pprof v0.0.0-20221118152302-e6195bd50e26 // indirect
	github.com/google/uuid v1.3.0 // indirect
)

module github.com/joschahenningsen/TUM-Live/worker

go 1.19

// Direct dependencies
require (
	github.com/getsentry/sentry-go v0.17.0
	github.com/golang/protobuf v1.5.2
	github.com/icza/gox v0.0.0-20230117093757-93f961aa2755
	github.com/iris-contrib/go.uuid v2.0.0+incompatible
	github.com/joschahenningsen/thumbgen v0.0.0-20220618164424-9fcc2beb0084
	github.com/otiai10/gosseract/v2 v2.4.0
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.23.1
	github.com/sirupsen/logrus v1.9.0
	github.com/tidwall/gjson v1.14.4
	github.com/u2takey/ffmpeg-go v0.4.1
	golang.org/x/sync v0.1.0
	google.golang.org/grpc v1.52.3
	google.golang.org/protobuf v1.28.1
)

require (
	github.com/bbalet/stopwords v1.0.0
	github.com/makasim/sentryhook v0.4.1
	github.com/pkg/profile v1.7.0
)

// Indirect dependencies
require (
	github.com/aws/aws-sdk-go v1.44.193 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20230110061619-bbe2e5e100de // indirect
	github.com/power-devops/perfstat v0.0.0-20221212215047-62379fc7944b // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/u2takey/go-utils v0.3.1
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	golang.org/x/text v0.6.0 // indirect
	google.golang.org/genproto v0.0.0-20230202175211-008b39050e57 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
)

require (
	github.com/felixge/fgprof v0.9.3 // indirect
	github.com/google/pprof v0.0.0-20230131232505-5a9e8f65f08f // indirect
	github.com/google/uuid v1.3.0 // indirect
)

module github.com/joschahenningsen/TUM-Live/worker

go 1.18

// Direct dependencies
require (
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/getsentry/sentry-go v0.13.0
	github.com/icza/gox v0.0.0-20220321141217-e2d488ab2fbc
	github.com/iris-contrib/go.uuid v2.0.0+incompatible
	github.com/joschahenningsen/thumbgen v0.0.0-20220618164424-9fcc2beb0084
	github.com/otiai10/gosseract/v2 v2.3.1
	github.com/robfig/cron/v3 v3.0.1
	github.com/shirou/gopsutil/v3 v3.22.5
	github.com/sirupsen/logrus v1.8.1
	github.com/tidwall/gjson v1.14.1
	github.com/u2takey/ffmpeg-go v0.4.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	google.golang.org/grpc v1.47.0
	google.golang.org/protobuf v1.28.0
)

require (
	github.com/bbalet/stopwords v1.0.0
	github.com/getsentry/sentry-go v0.13.0
	github.com/joschahenningsen/thumbgen v0.0.0-20220601131629-d049c9087bf3
	github.com/makasim/sentryhook v0.4.0
	github.com/pkg/profile v1.6.0
)

// Indirect dependencies
require (
	github.com/aws/aws-sdk-go v1.44.26 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220517141722-cf486979b281 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.5.0 // indirect
	github.com/u2takey/go-utils v0.3.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	golang.org/x/net v0.0.0-20220425223048-2871e0cb64e4 // indirect
	golang.org/x/sys v0.0.0-20220519141025-dcacdad47464 // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20220601144221-27df5f98adab // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

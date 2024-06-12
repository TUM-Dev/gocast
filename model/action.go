package model

import "time"

type Action struct {
	ID        string
	Type      string
	Runner    *Runner
	Start     time.Time
	End       time.Time
	Completed bool
	Workload  int
}

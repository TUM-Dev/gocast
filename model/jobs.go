package model

type Job struct {
	ID          string
	Actions     []*Action
	Runners     []*Runner
	Description string
	Completed   bool
}

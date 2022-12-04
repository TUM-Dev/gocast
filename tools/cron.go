package tools

import "github.com/robfig/cron/v3"

type CronService struct {
	cronJobs map[string]func()
	cron     *cron.Cron
}

// Cron is the global CronService
var Cron *CronService

// InitCronService creates an instance of CronService
func InitCronService() {
	Cron = &CronService{
		cronJobs: make(map[string]func(), 0),
		cron:     cron.New(),
	}
}

// AddFunc creates a cronJob fn running at the interval specified by spec. The job can be referenced by name.
func (c *CronService) AddFunc(name string, fn func(), spec string) error {
	c.cronJobs[name] = fn
	_, err := c.cron.AddFunc(spec, fn)
	return err
}

// Run starts the CronService
func (c *CronService) Run() {
	c.cron.Start()
}

// RunJob executes the cronJob identified by name even when it's not due.
// Invalid names are ignored silently.
func (c *CronService) RunJob(name string) {
	if job, ok := c.cronJobs[name]; ok {
		go job()
	}
}

// ListCronJobs returns a []string with the names of all cronjobs
func (c *CronService) ListCronJobs() []string {
	var l []string
	for job := range c.cronJobs {
		l = append(l, job)
	}
	return l
}

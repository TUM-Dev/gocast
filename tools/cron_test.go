package tools

import (
	"sort"
	"testing"
)

// newTestCronService builds a CronService without touching the global Cron, so these
// tests cannot leak state into the rest of the package.
func newTestCronService(t *testing.T) *CronService {
	t.Helper()

	before := Cron
	InitCronService()
	c := Cron
	t.Cleanup(func() { Cron = before })
	return c
}

func TestCronServiceAddFunc(t *testing.T) {
	t.Run("registers a job under a valid spec", func(t *testing.T) {
		c := newTestCronService(t)

		if err := c.AddFunc("collect", func() {}, "0 0 * * *"); err != nil {
			t.Fatalf("AddFunc returned %v, want nil", err)
		}
		if got := c.ListCronJobs(); len(got) != 1 || got[0] != "collect" {
			t.Errorf("ListCronJobs() = %v, want [collect]", got)
		}
	})

	t.Run("accepts a descriptor spec", func(t *testing.T) {
		c := newTestCronService(t)

		if err := c.AddFunc("hourly", func() {}, "@every 1h"); err != nil {
			t.Errorf("AddFunc returned %v, want nil", err)
		}
	})

	// An invalid spec is a config typo at startup. The scheduler rejects it, but the
	// name is registered before the spec is parsed, so the job stays reachable by name
	// even though it will never fire on a schedule.
	t.Run("an invalid spec errors yet still leaves the job listed", func(t *testing.T) {
		c := newTestCronService(t)

		if err := c.AddFunc("broken", func() {}, "not a cron spec"); err == nil {
			t.Fatal("AddFunc returned nil, want an error")
		}
		if got := c.ListCronJobs(); len(got) != 1 || got[0] != "broken" {
			t.Errorf("ListCronJobs() = %v, want [broken]", got)
		}
	})

	t.Run("re-adding a name replaces the function it resolves to", func(t *testing.T) {
		c := newTestCronService(t)

		ran := make(chan string, 2)
		for _, which := range []string{"first", "second"} {
			if err := c.AddFunc("job", func() { ran <- which }, "0 0 * * *"); err != nil {
				t.Fatalf("AddFunc(%q) returned %v", which, err)
			}
		}

		c.RunJob("job")
		if got := <-ran; got != "second" {
			t.Errorf("ran %q, want %q", got, "second")
		}
		if got := c.ListCronJobs(); len(got) != 1 {
			t.Errorf("ListCronJobs() = %v, want a single entry", got)
		}
	})
}

func TestCronServiceListCronJobs(t *testing.T) {
	t.Run("is empty before anything is added", func(t *testing.T) {
		c := newTestCronService(t)

		if got := c.ListCronJobs(); len(got) != 0 {
			t.Errorf("ListCronJobs() = %v, want empty", got)
		}
	})

	t.Run("lists every registered name", func(t *testing.T) {
		c := newTestCronService(t)

		for _, name := range []string{"a", "b", "c"} {
			if err := c.AddFunc(name, func() {}, "0 0 * * *"); err != nil {
				t.Fatalf("AddFunc(%q) returned %v", name, err)
			}
		}

		got := c.ListCronJobs()
		sort.Strings(got)
		want := []string{"a", "b", "c"}
		if len(got) != len(want) {
			t.Fatalf("ListCronJobs() = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("ListCronJobs() = %v, want %v", got, want)
			}
		}
	})
}

// RunJob backs an admin route that passes the name straight from the request, so an
// unknown name must be a no-op rather than a panic, and the scheduler need not be
// running for a manual trigger to work.
func TestCronServiceRunJob(t *testing.T) {
	t.Run("runs a registered job without starting the scheduler", func(t *testing.T) {
		c := newTestCronService(t)

		ran := make(chan struct{})
		if err := c.AddFunc("collect", func() { close(ran) }, "0 0 * * *"); err != nil {
			t.Fatalf("AddFunc returned %v", err)
		}

		c.RunJob("collect")
		<-ran
	})

	t.Run("ignores an unknown name", func(t *testing.T) {
		c := newTestCronService(t)

		ran := make(chan struct{}, 1)
		if err := c.AddFunc("collect", func() { ran <- struct{}{} }, "0 0 * * *"); err != nil {
			t.Fatalf("AddFunc returned %v", err)
		}

		c.RunJob("nonexistent")
		c.RunJob("")

		select {
		case <-ran:
			t.Error("an unknown name triggered a registered job")
		default:
		}
	})

	t.Run("runs a job whose spec was rejected", func(t *testing.T) {
		c := newTestCronService(t)

		ran := make(chan struct{})
		if err := c.AddFunc("broken", func() { close(ran) }, "nonsense"); err == nil {
			t.Fatal("AddFunc returned nil, want an error")
		}

		c.RunJob("broken")
		<-ran
	})
}

func TestInitCronService(t *testing.T) {
	// A nil map here would panic on the first AddFunc at startup.
	c := newTestCronService(t)

	if c == nil || Cron != c {
		t.Fatal("InitCronService did not set the global Cron")
	}
	if c.cronJobs == nil {
		t.Error("cronJobs map is nil")
	}
	if c.cron == nil {
		t.Error("cron scheduler is nil")
	}
}

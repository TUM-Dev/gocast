package psutil

import (
	"context"
	"fmt"
	"time"

	"github.com/icza/gox/fmtx"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"golang.org/x/sync/errgroup"
)

type Psutil struct {
	diskPath string

	CpuPercent float64

	MemTotal     uint64
	MemAvailable uint64

	DiskTotal   uint64
	DiskPercent float64
	DiskUsed    uint64
}

var (
	ErrCPUInvalid = fmt.Errorf("len (cpu.Percent) != 1")
)

func New() *Psutil {
	return &Psutil{diskPath: "/"}
}

func NewWithPath(path string) *Psutil {
	return &Psutil{diskPath: path}
}

func (p *Psutil) Update() error {
	g, _ := errgroup.WithContext(context.Background())
	g.Go(p.getCpu)
	g.Go(p.getMem)
	g.Go(p.getDisk)
	return g.Wait()
}

func (p *Psutil) getMem() error {
	memory, err := mem.VirtualMemory()
	if err != nil {
		return err
	}
	p.MemAvailable = memory.Available
	p.MemTotal = memory.Total
	return nil
}

func (p *Psutil) getCpu() error {
	percent, err := cpu.Percent(time.Second*5, false)
	if err != nil {
		return err
	}
	if len(percent) != 1 {
		return ErrCPUInvalid
	}
	p.CpuPercent = percent[0]
	return nil
}

func (p *Psutil) getDisk() error {
	usage, err := disk.Usage("/")
	if err != nil {
		return err
	}
	p.DiskUsed = usage.Used
	p.DiskTotal = usage.Total
	p.DiskPercent = usage.UsedPercent
	return nil
}

func (p *Psutil) GetCpuStr() string {
	return fmt.Sprintf("%.f%%", p.CpuPercent)
}

func (p *Psutil) GetMemStr() string {
	if p.MemAvailable == 0 || p.MemTotal == 0 {
		return "unknown"
	}
	return fmt.Sprintf("%sM/%sM (%.f%%)",
		fmtx.FormatInt(int64(p.MemAvailable/1e+6), 3, '.'),
		fmtx.FormatInt(int64(p.MemTotal/1e+6), 3, '.'),
		(1-(float64(p.MemAvailable)/float64(p.MemTotal)))*100)
}

func (p *Psutil) GetDiskStr() string {
	return fmt.Sprintf("%dG/%dG (%.f%%)", p.DiskUsed/1e+9, p.DiskTotal/1e+9, p.DiskPercent)
}

func (p *Psutil) String() string {
	return fmt.Sprintf("CPU: [%s], Mem: [%s], Disk: [%s]",
		p.GetCpuStr(), p.GetMemStr(), p.GetDiskStr())
}

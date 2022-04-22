package vmstat

import "testing"

func TestVmStat_GetCpuStr(t *testing.T) {
	s := New()
	s.CpuPercent = 42
	if s.GetCpuStr() != "42%" {
		t.Error("GetCpuStr() failed")
	}
}

func TestVmStat_GetMemStr(t *testing.T) {
	s := New()
	s.MemAvailable = 28690272256
	s.MemTotal = 33638785024
	if s.GetMemStr() != "28.690M/33.638M (15%)" {
		t.Error("GetMemStr() failed, got:", s.GetMemStr(), "expected: 28.690M/33.638M (15%)")
	}
}

func TestVmStat_GetDiskStr(t *testing.T) {
	s := New()
	s.DiskTotal = 974437085184
	s.DiskUsed = 287634259968
	s.DiskPercent = 31.100071498992595
	if s.GetDiskStr() != "287G/974G (31%)" {
		t.Error("GetDiskStr() failed, got:", s.GetDiskStr(), "expected: 28.690M/33.638M (15%)")
	}
}

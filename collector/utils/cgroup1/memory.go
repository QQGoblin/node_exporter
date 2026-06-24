package cgroup1

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/containerd/cgroups/v3/cgroup1"
)

type MemoryStat struct {
	Cache                   uint64
	RSS                     uint64
	RSSHuge                 uint64
	Shmem                   uint64
	MappedFile              uint64
	Dirty                   uint64
	Writeback               uint64
	Swap                    uint64
	SwapCached              uint64
	PgPgIn                  uint64
	PgPgOut                 uint64
	PgFault                 uint64
	PgMajFault              uint64
	InactiveAnon            uint64
	ActiveAnon              uint64
	InactiveFile            uint64
	ActiveFile              uint64
	Unevictable             uint64
	HierarchicalMemoryLimit uint64
	HierarchicalMemswLimit  uint64
	TotalCache              uint64
	TotalRSS                uint64
	TotalRSSHuge            uint64
	TotalShmem              uint64
	TotalMappedFile         uint64
	TotalDirty              uint64
	TotalWriteback          uint64
	TotalSwap               uint64
	TotalSwapCached         uint64
	TotalPgPgIn             uint64
	TotalPgPgOut            uint64
	TotalPgFault            uint64
	TotalPgMajFault         uint64
	TotalInactiveAnon       uint64
	TotalActiveAnon         uint64
	TotalInactiveFile       uint64
	TotalActiveFile         uint64
	TotalUnevictable        uint64
	Raw                     map[string]uint64
}

func MemoryStatByPid(pid int) (*MemoryStat, error) {
	statPath, err := MemoryStatPath(pid)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(statPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open memory.stat for pid %d: %w", pid, err)
	}
	defer file.Close()

	stat, err := ParseCgroupV1MemoryStat(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse memory.stat for pid %d: %w", pid, err)
	}

	return stat, nil
}

func MemoryStatPath(pid int) (string, error) {
	cgroupPath, err := cgroup1.PidPath(pid)(cgroup1.Memory)
	if err != nil {
		return "", fmt.Errorf("failed to find memory cgroup path for pid %d: %w", pid, err)
	}

	subsystems, err := cgroup1.Default()
	if err != nil {
		return "", fmt.Errorf("failed to load cgroup v1 subsystems: %w", err)
	}

	for _, subsystem := range subsystems {
		if subsystem.Name() != cgroup1.Memory {
			continue
		}

		pather, ok := subsystem.(interface {
			Path(string) string
		})
		if !ok {
			return "", fmt.Errorf("memory cgroup subsystem cannot resolve filesystem path")
		}
		return filepath.Join(pather.Path(cgroupPath), "memory.stat"), nil
	}

	return "", fmt.Errorf("memory cgroup subsystem not found for pid %d", pid)
}

func ParseCgroupV1MemoryStat(r io.Reader) (*MemoryStat, error) {
	raw := make(map[string]uint64)
	scanner := bufio.NewScanner(r)
	line := 1

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 0 {
			line++
			continue
		}
		if len(fields) != 2 {
			return nil, fmt.Errorf("line %d: invalid memory.stat format", line)
		}

		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: invalid value for %s: %w", line, fields[0], err)
		}
		raw[fields[0]] = value
		line++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &MemoryStat{
		Cache:                   raw["cache"],
		RSS:                     raw["rss"],
		RSSHuge:                 raw["rss_huge"],
		Shmem:                   raw["shmem"],
		MappedFile:              raw["mapped_file"],
		Dirty:                   raw["dirty"],
		Writeback:               raw["writeback"],
		Swap:                    raw["swap"],
		SwapCached:              raw["swapcached"],
		PgPgIn:                  raw["pgpgin"],
		PgPgOut:                 raw["pgpgout"],
		PgFault:                 raw["pgfault"],
		PgMajFault:              raw["pgmajfault"],
		InactiveAnon:            raw["inactive_anon"],
		ActiveAnon:              raw["active_anon"],
		InactiveFile:            raw["inactive_file"],
		ActiveFile:              raw["active_file"],
		Unevictable:             raw["unevictable"],
		HierarchicalMemoryLimit: raw["hierarchical_memory_limit"],
		HierarchicalMemswLimit:  raw["hierarchical_memsw_limit"],
		TotalCache:              raw["total_cache"],
		TotalRSS:                raw["total_rss"],
		TotalRSSHuge:            raw["total_rss_huge"],
		TotalShmem:              raw["total_shmem"],
		TotalMappedFile:         raw["total_mapped_file"],
		TotalDirty:              raw["total_dirty"],
		TotalWriteback:          raw["total_writeback"],
		TotalSwap:               raw["total_swap"],
		TotalSwapCached:         raw["total_swapcached"],
		TotalPgPgIn:             raw["total_pgpgin"],
		TotalPgPgOut:            raw["total_pgpgout"],
		TotalPgFault:            raw["total_pgfault"],
		TotalPgMajFault:         raw["total_pgmajfault"],
		TotalInactiveAnon:       raw["total_inactive_anon"],
		TotalActiveAnon:         raw["total_active_anon"],
		TotalInactiveFile:       raw["total_inactive_file"],
		TotalActiveFile:         raw["total_active_file"],
		TotalUnevictable:        raw["total_unevictable"],
		Raw:                     raw,
	}, nil
}

func (s *MemoryStat) Value(name string) (uint64, bool) {
	if s == nil || s.Raw == nil {
		return 0, false
	}
	value, ok := s.Raw[name]
	return value, ok
}

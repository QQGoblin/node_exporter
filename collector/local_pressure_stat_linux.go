// Copyright 2026 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// LOCAL CUSTOMIZATION NOTICE:
// This collector is a project-local addition for this repository and is not part
// of upstream node_exporter. It exposes the openEuler-specific /proc/pressure/stat
// extension file.

//go:build !nolocal_pressure_stat

package collector

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

const pressureStatSubsystem = "pressure_stat"

type pressureStatValues struct {
	total uint64
}

type pressureStatResource struct {
	name string
	some *pressureStatValues
	full *pressureStatValues
}

type pressureStatCollector struct {
	total  *prometheus.Desc
	logger *slog.Logger
}

func init() {
	registerCollector("pressure_stat", defaultDisabled, NewPressureStatCollector)
}

// NewPressureStatCollector returns a Collector exposing the openEuler /proc/pressure/stat extension.
func NewPressureStatCollector(logger *slog.Logger) (Collector, error) {
	return &pressureStatCollector{
		total: prometheus.NewDesc(
			prometheus.BuildFQName(namespace, pressureStatSubsystem, "seconds_total"),
			"Total pressure stall time in seconds reported by /proc/pressure/stat.",
			[]string{"resource", "scope"}, nil,
		),
		logger: logger,
	}, nil
}

func (c *pressureStatCollector) Update(ch chan<- prometheus.Metric) error {
	resources, err := readPressureStat(procFilePath("pressure/stat"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c.logger.Debug("pressure stat information is unavailable, you need openEuler /proc/pressure/stat support")
			return ErrNoData
		}

		return fmt.Errorf("failed to retrieve pressure stat information: %w", err)
	}

	foundResources := 0
	for _, resource := range resources {
		c.logger.Debug("collecting local pressure statistics for resource", "resource", resource.name)
		for scope, values := range map[string]*pressureStatValues{"some": resource.some, "full": resource.full} {
			if values == nil {
				continue
			}

			ch <- prometheus.MustNewConstMetric(c.total, prometheus.CounterValue, float64(values.total)/1000.0/1000.0, resource.name, scope)
			foundResources++
		}
	}

	if foundResources == 0 {
		c.logger.Debug("pressure stat information returned no data")
		return ErrNoData
	}

	return nil
}

func readPressureStat(path string) ([]pressureStatResource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return parsePressureStatFile(file)
}

func parsePressureStatFile(r io.Reader) ([]pressureStatResource, error) {
	scanner := bufio.NewScanner(r)
	resources := make([]pressureStatResource, 0)
	currentResource := -1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 1 && !strings.Contains(fields[0], "=") {
			resources = append(resources, pressureStatResource{name: fields[0]})
			currentResource = len(resources) - 1
			continue
		}

		if currentResource < 0 {
			return nil, fmt.Errorf("missing resource header before line %q", line)
		}

		scope, values, err := parsePressureStatRecord(resources[currentResource].name, fields)
		if err != nil {
			return nil, err
		}

		switch scope {
		case "some":
			resources[currentResource].some = values
		case "full":
			resources[currentResource].full = values
		default:
			return nil, fmt.Errorf("unexpected pressure stat scope %q for resource %q", scope, resources[currentResource].name)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(resources, func(i, j int) bool {
		return resources[i].name < resources[j].name
	})

	return resources, nil
}

func parsePressureStatRecord(resource string, fields []string) (string, *pressureStatValues, error) {
	if len(fields) < 5 {
		return "", nil, fmt.Errorf("unexpected pressure stat record for resource %q: %q", resource, strings.Join(fields, " "))
	}

	scope := fields[0]
	if scope != "some" && scope != "full" {
		return "", nil, fmt.Errorf("unexpected pressure stat scope %q for resource %q", scope, resource)
	}

	values := &pressureStatValues{}

	var hasTotal bool

	for _, field := range fields[1:] {
		key, value, ok := strings.Cut(field, "=")
		if !ok {
			return "", nil, fmt.Errorf("unexpected pressure stat field %q for resource %q", field, resource)
		}

		switch key {
		case "total":
			parsed, err := strconv.ParseUint(value, 10, 64)
			if err != nil {
				return "", nil, fmt.Errorf("parse total for resource %q: %w", resource, err)
			}
			values.total = parsed
			hasTotal = true
		}
	}

	if !hasTotal {
		return "", nil, fmt.Errorf("incomplete pressure stat record for resource %q scope %q", resource, scope)
	}

	return scope, values, nil
}

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
// of upstream node_exporter. It exposes DAMON reclaim statistics from
// /sys/module/damon_reclaim/parameters.

//go:build !nolocal_damon_reclaim

package collector

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/prometheus/client_golang/prometheus"
)

const damonReclaimSubsystem = "damon_reclaim"

var damonReclaimMetrics = []struct {
	fileName        string
	metricName      string
	help            string
	valueType       prometheus.ValueType
	scaleByAddrUnit bool
}{
	{
		fileName:        "bytes_reclaimed_regions",
		metricName:      "bytes_total",
		help:            "Total bytes of memory successfully reclaimed by DAMON reclaim.",
		valueType:       prometheus.CounterValue,
		scaleByAddrUnit: true,
	},
	{
		fileName:        "bytes_reclaim_tried_regions",
		metricName:      "bytes_tried_total",
		help:            "Total bytes of memory DAMON reclaim attempted to reclaim.",
		valueType:       prometheus.CounterValue,
		scaleByAddrUnit: true,
	},
	{
		fileName:   "nr_quota_exceeds",
		metricName: "quota_exceeds_total",
		help:       "Total number of times DAMON reclaim quota limits were exceeded.",
		valueType:  prometheus.CounterValue,
	},
	{
		fileName:   "nr_reclaimed_regions",
		metricName: "regions_total",
		help:       "Total number of memory regions successfully reclaimed by DAMON reclaim.",
		valueType:  prometheus.CounterValue,
	},
	{
		fileName:   "nr_reclaim_tried_regions",
		metricName: "tried_regions_total",
		help:       "Total number of memory regions DAMON reclaim attempted to reclaim.",
		valueType:  prometheus.CounterValue,
	},
}

type damonReclaimCollector struct {
	metricDescs map[string]*prometheus.Desc
	logger      *slog.Logger
}

func init() {
	registerCollector(damonReclaimSubsystem, defaultDisabled, NewDamonReclaimCollector)
}

// NewDamonReclaimCollector returns a Collector exposing the /sys/module/damon_reclaim/parameters files.
func NewDamonReclaimCollector(logger *slog.Logger) (Collector, error) {
	descs := make(map[string]*prometheus.Desc, len(damonReclaimMetrics))
	for _, metric := range damonReclaimMetrics {
		descs[metric.fileName] = prometheus.NewDesc(
			prometheus.BuildFQName(namespace, damonReclaimSubsystem, metric.metricName),
			metric.help,
			nil, nil,
		)
	}

	return &damonReclaimCollector{
		metricDescs: descs,
		logger:      logger,
	}, nil
}

func (c *damonReclaimCollector) Update(ch chan<- prometheus.Metric) error {
	parametersPath := sysFilePath(filepath.Join("module", damonReclaimSubsystem, "parameters"))

	if _, err := os.Stat(parametersPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to read damon_reclaim status: %w", err)
	}

	for _, metric := range damonReclaimMetrics {
		value, err := readUintFromFile(filepath.Join(parametersPath, metric.fileName))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				c.logger.Debug("damon reclaim parameter is unavailable", "file", metric.fileName, "path", parametersPath)
				return ErrNoData
			}
			return fmt.Errorf("failed to read DAMON reclaim parameter %q: %w", metric.fileName, err)
		}

		ch <- prometheus.MustNewConstMetric(c.metricDescs[metric.fileName], metric.valueType, float64(value))
	}

	return nil
}

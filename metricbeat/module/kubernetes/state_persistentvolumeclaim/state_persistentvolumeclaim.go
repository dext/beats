// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package state_persistentvolumeclaim

import (
	"fmt"

	"github.com/elastic/beats/v7/metricbeat/module/kubernetes/util"
	"github.com/elastic/elastic-agent-autodiscover/kubernetes"

	p "github.com/elastic/beats/v7/metricbeat/helper/prometheus"
	"github.com/elastic/beats/v7/metricbeat/mb"
	k8smod "github.com/elastic/beats/v7/metricbeat/module/kubernetes"
)

func init() {
	mb.Registry.MustAddMetricSet("kubernetes", "state_persistentvolumeclaim",
		NewpersistentvolumeclaimMetricSet,
		mb.WithHostParser(p.HostParser))
}

// persistentvolumeclaimMetricSet is a prometheus based MetricSet that looks for
// mb.ModuleDataKey prefixed fields and puts then at the module level
type persistentvolumeclaimMetricSet struct {
	mb.BaseMetricSet
	prometheus p.Prometheus
	mapping    *p.MetricsMapping
	enricher   util.Enricher
	mod        k8smod.Module
}

// NewpersistentvolumeclaimMetricSet returns a prometheus based metricset for Persistent Volumes
func NewpersistentvolumeclaimMetricSet(base mb.BaseMetricSet) (mb.MetricSet, error) {
	prometheus, err := p.NewPrometheusClient(base)
	if err != nil {
		return nil, err
	}
	mod, ok := base.Module().(k8smod.Module)
	if !ok {
		return nil, fmt.Errorf("must be child of kubernetes module")
	}
	return &persistentvolumeclaimMetricSet{
		BaseMetricSet: base,
		prometheus:    prometheus,
		mod:           mod,
		enricher:      util.NewResourceMetadataEnricher(base, &kubernetes.PersistentVolumeClaim{}, mod.GetPerfMetricsCache(), false),
		mapping: &p.MetricsMapping{
			Metrics: map[string]p.MetricMap{

				"kube_persistentvolumeclaim_access_mode": p.LabelMetric("access_mode", "access_mode"),
				"kube_persistentvolumeclaim_info":        p.InfoMetric(),
				"kube_persistentvolumeclaim_labels": p.ExtendedInfoMetric(
					p.Configuration{
						StoreNonMappedLabels:     true,
						NonMappedLabelsPlacement: mb.ModuleDataKey + ".labels",
						MetricProcessingOptions:  []p.MetricOption{p.OpLabelKeyPrefixRemover("label_")},
					}),
				"kube_persistentvolumeclaim_resource_requests_storage_bytes": p.Metric("request_storage.bytes"),
				"kube_persistentvolumeclaim_status_phase":                    p.LabelMetric("phase", "phase"),
			},
			Labels: map[string]p.LabelMap{
				"namespace":             p.KeyLabel(mb.ModuleDataKey + ".namespace"),
				"persistentvolumeclaim": p.KeyLabel("name"),
				"storageclass":          p.Label("storage_class"),
				"volumename":            p.Label("volume_name"),
			},
		},
	}, nil
}

// Fetch prometheus metrics and treats those prefixed by mb.ModuleDataKey as
// module rooted fields at the event that gets reported
func (m *persistentvolumeclaimMetricSet) Fetch(reporter mb.ReporterV2) {
	m.enricher.Start()

	families, err := m.mod.GetStateMetricsFamilies(m.prometheus)
	if err != nil {
		m.Logger().Error(err)
		reporter.Error(err)
		return
	}
	events, err := m.prometheus.ProcessMetrics(families, m.mapping)
	if err != nil {
		m.Logger().Error(err)
		reporter.Error(err)
		return
	}

	m.enricher.Enrich(events)
	for _, event := range events {

		e, err := util.CreateEvent(event, "kubernetes.persistentvolumeclaim")
		if err != nil {
			m.Logger().Error(err)
		}

		if reported := reporter.Event(e); !reported {
			m.Logger().Debug("error trying to emit event")
			return
		}
	}

}

// Close stops this metricset
func (m *persistentvolumeclaimMetricSet) Close() error {
	m.enricher.Stop()
	return nil
}

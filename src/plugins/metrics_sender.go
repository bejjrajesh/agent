/**
 * Copyright (c) F5, Inc.
 *
 * This source code is licensed under the Apache License, Version 2.0 license found in the
 * LICENSE file in the root directory of this source tree.
 */

package plugins

import (
	"context"
	"github.com/nginx/agent/sdk/v2"
	agent_config "github.com/nginx/agent/sdk/v2/agent/config"
	"github.com/nginx/agent/sdk/v2/client"
	"github.com/nginx/agent/sdk/v2/proto"
	models "github.com/nginx/agent/sdk/v2/proto/events"
	"github.com/nginx/agent/v2/src/core"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
)

type MetricsSender struct {
	reporter      client.MetricReporter
	pipeline      core.MessagePipeInterface
	ctx           context.Context
	started       *atomic.Bool
	readyToSend   *atomic.Bool
	readyToSendMu sync.RWMutex
}

func NewMetricsSender(reporter client.MetricReporter) *MetricsSender {
	return &MetricsSender{
		reporter:    reporter,
		started:     atomic.NewBool(false),
		readyToSend: atomic.NewBool(false),
	}
}

func (r *MetricsSender) Init(pipeline core.MessagePipeInterface) {
	if r.started.Load() {
		return
	}
	r.started.Toggle()
	r.pipeline = pipeline
	r.ctx = pipeline.Context()
	log.Infof("MetricsSender initializing %v %v", r.started, r.readyToSend)
}

func (r *MetricsSender) Close() {
	log.Info("MetricsSender is wrapping up")
	r.readyToSendMu.Lock()
	r.started.Store(false)
	r.readyToSend.Store(false)
	defer r.readyToSendMu.Unlock()
}

func (r *MetricsSender) Info() *core.Info {
	return core.NewInfo(agent_config.FeatureMetricsSender, "v0.0.1")
}

func (r *MetricsSender) Process(msg *core.Message) {
	//if msg.Exact(core.AgentConnected) {
	//	log.Debugf("MetricsSender AgentConnected Before: %v", r.readyToSend)
	//	r.readyToSend.Store(true)
	//	log.Debugf("MetricsSender AgentConnected After %v", r.readyToSend)
	//	return
	//}

	if msg.Exact(core.CommMetrics) {
		payloads, ok := msg.Data().([]core.Payload)
		if !ok {
			log.Warnf("Failed to coerce Message to []Payload: %v", msg.Data())
			return
		}
		for _, p := range payloads {
			if !r.readyToSend.Load() {
				log.Debugf("metrics_sender is not ready to send the metrics")
				continue
			}

			switch report := p.(type) {
			case *proto.MetricsReport:
				message := client.MessageFromMetrics(report)
				log.Debugf("metrics_sender sending the metrics report")
				err := r.reporter.Send(r.ctx, message)
				if err != nil {
					log.Errorf("Failed to send MetricsReport: %v", err)
				} else {
					r.pipeline.Process(core.NewMessage(core.MetricReportSent, nil))
				}
			case *models.EventReport:
				log.Debugf("metrics_sender sending the events report")
				err := r.reporter.Send(r.ctx, client.MessageFromEvents(report))
				if err != nil {
					l := len(report.Events)
					var sb strings.Builder
					for i := 0; i < l-1; i++ {
						sb.WriteString(report.Events[i].GetSecurityViolationEvent().SupportID)
						sb.WriteString(", ")
					}
					sb.WriteString(report.Events[l-1].GetSecurityViolationEvent().SupportID)
					log.Errorf("Failed to send EventReport with error: %v, supportID list: %s", err, sb.String())
				}
			}
		}
	} else if msg.Exact(core.AgentConfigChanged) {
		switch config := msg.Data().(type) {
		case *proto.AgentConfig:
			r.metricSenderBackoff(config)
		default:
			log.Warnf("metrics sender expected %T type, but got: %T", &proto.AgentConfig{}, msg.Data())
		}
	}
}

func (r *MetricsSender) metricSenderBackoff(agentConfig *proto.AgentConfig) {
	log.Debugf("update metric reporter client configuration to %+v", agentConfig)
	if agentConfig.Details.Features != nil {
		if r.isFeatureEnabled(agentConfig) {
			r.readyToSendMu.Lock()
			r.readyToSend.Store(true)
			r.readyToSendMu.Unlock()
		} else {
			r.readyToSendMu.Lock()
			r.readyToSend.Store(false)
			r.readyToSendMu.Unlock()
		}
	}
	if agentConfig.GetDetails() == nil || agentConfig.GetDetails().GetServer() == nil || agentConfig.GetDetails().GetServer().GetBackoff() == nil {
		log.Debug("not updating metric reporter client configuration as new Agent backoff settings is nil")
		return
	}

	backOffSettings := sdk.ConvertBackOffSettings(agentConfig.GetDetails().GetServer().GetBackoff())
	r.reporter.WithBackoffSettings(backOffSettings)
}

func (r *MetricsSender) Subscriptions() []string {
	return []string{core.CommMetrics, core.AgentConnected, core.AgentConfigChanged}
}

func (r *MetricsSender) isFeatureEnabled(agentConfig *proto.AgentConfig) bool {
	var isFeatureEnabled bool
	if agentConfig.Details.Features != nil {
		for _, feature := range agentConfig.Details.Features {
			if feature == agent_config.FeatureMetricsSender {
				isFeatureEnabled = true
				break
			}
		}
	}
	return isFeatureEnabled
}

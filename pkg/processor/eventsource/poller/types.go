/*
Copyright 2017 The Nuclio Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package poller

import (
	"github.com/nuclio/nuclio-sdk"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"

	"github.com/spf13/viper"
)

type Configuration struct {
	eventsource.Configuration
	IntervalMs     int
	MaxBatchSize   int
	MaxBatchWaitMs int
}

func NewConfiguration(configuration *viper.Viper) *Configuration {
	return &Configuration{
		Configuration:  *eventsource.NewConfiguration(configuration),
		IntervalMs:     configuration.GetInt("interval_ms"),
		MaxBatchSize:   configuration.GetInt("max_batch_size"),
		MaxBatchWaitMs: configuration.GetInt("max_batch_wait_ms"),
	}
}

type Poller interface {
	eventsource.EventSource

	// read new events into a channel
	GetNewEvents(chan nuclio.Event) error

	// handle a set of events that were processed
	PostProcessEvents([]nuclio.Event, []interface{}, []error)
}

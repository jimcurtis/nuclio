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

package rabbitmq

import (
	"time"

	"github.com/nuclio/nuclio/pkg/errors"
	"github.com/nuclio/nuclio/pkg/processor/eventsource"
	"github.com/nuclio/nuclio/pkg/processor/worker"
	"github.com/nuclio/nuclio/pkg/util/common"

	"github.com/nuclio/nuclio-sdk"
	"github.com/streadway/amqp"
)

type rabbitMq struct {
	eventsource.AbstractEventSource
	event                      Event
	configuration              *Configuration
	brokerConn                 *amqp.Connection
	brokerChannel              *amqp.Channel
	brokerQueue                amqp.Queue
	brokerInputMessagesChannel <-chan amqp.Delivery
	worker                     *worker.Worker
}

func newEventSource(parentLogger nuclio.Logger,
	workerAllocator worker.WorkerAllocator,
	configuration *Configuration) (eventsource.EventSource, error) {

	newEventSource := rabbitMq{
		AbstractEventSource: eventsource.AbstractEventSource{
			Logger:          parentLogger.GetChild("rabbitMq").(nuclio.Logger),
			WorkerAllocator: workerAllocator,
			Class:           "async",
			Kind:            "rabbitMq",
		},
		configuration: configuration,
	}

	return &newEventSource, nil
}

func (rmq *rabbitMq) Start(checkpoint eventsource.Checkpoint) error {
	var err error

	rmq.Logger.InfoWith("Starting", "brokerUrl", rmq.configuration.BrokerUrl)

	// get a worker, we'll be using this one always
	rmq.worker, err = rmq.WorkerAllocator.Allocate(10 * time.Second)
	if err != nil {
		return errors.Wrap(err, "Failed to allocate worker")
	}

	if err := rmq.createBrokerResources(); err != nil {
		return errors.Wrap(err, "Failed to create broker resources")
	}

	// start listening for published messages
	go rmq.handleBrokerMessages()

	return nil
}

func (rmq *rabbitMq) Stop(force bool) (eventsource.Checkpoint, error) {

	// TODO
	return nil, nil
}

func (rmq *rabbitMq) GetConfig() map[string]interface{} {
	return common.StructureToMap(rmq.configuration)
}

func (rmq *rabbitMq) createBrokerResources() error {
	var err error

	rmq.brokerConn, err = amqp.Dial(rmq.configuration.BrokerUrl)
	if err != nil {
		return errors.Wrap(err, "Failed to create connection to broker")
	}

	rmq.brokerChannel, err = rmq.brokerConn.Channel()
	if err != nil {
		return errors.Wrap(err, "Failed to create channel")
	}

	rmq.brokerQueue, err = rmq.brokerChannel.QueueDeclare(
		"foo", // queue name (account  + function name)
		false, // durable  TBD: change to true if/when we bind to persistent storage
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return errors.Wrap(err, "Failed to create queue")
	}

	err = rmq.brokerChannel.QueueBind(
		rmq.brokerQueue.Name, // queue name
		"foo",                // routing key
		rmq.configuration.BrokerExchangeName, // exchange
		false,
		nil)
	if err != nil {
		return errors.Wrap(err, "Failed to bind to queue")
	}

	rmq.brokerInputMessagesChannel, err = rmq.brokerChannel.Consume(
		rmq.brokerQueue.Name, // queue
		"",                   // consumer
		false,                // auto-ack
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	if err != nil {
		return errors.Wrap(err, "Failed to start consuming messages")
	}

	return nil
}

func (rmq *rabbitMq) handleBrokerMessages() {
	for {
		select {
		case message := <-rmq.brokerInputMessagesChannel:

			// bind to delivery
			rmq.event.message = &message

			// submit to worker
			_, submitError, processError := rmq.SubmitEventToWorker(&rmq.event, nil, 10*time.Second)

			// TODO: do something with response and process error?
			rmq.Logger.DebugWith("Processed message", "processError", processError)

			// ack the message if we didn't fail to submit
			if submitError == nil {
				message.Ack(false)
			} else {
				errors.Wrap(submitError, "Failed to submit to worker")
			}
		}
	}
}

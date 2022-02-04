package mycscloud

import (
	"context"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-common/events"
	"github.com/mevansam/goutils/logger"
)

type EventPublisher struct {
	config config.Config
	apiUrl, 
	subUrl string
}

func NewEventPublisher(apiUrl, subUrl string, config config.Config) *EventPublisher {

	return &EventPublisher{
		config: config,
		apiUrl: apiUrl,
		subUrl: subUrl,
	}
}

func (p *EventPublisher) PostMeasurementEvents(cloudEvents []*cloudevents.Event) ([]events.CloudEventError, error) {

	var (
		err error
		ok  bool

		deviceID,
		eventSource string
		sourceUrn strings.Builder
	)
	if !p.config.AuthContext().IsLoggedIn() {
		logger.TraceMessage("EventPublisher.PostMeasurementEvents(): Client is not logged in. Measurement events will be not be recorded.")
		return nil, nil
	}
	apiClient := api.NewGraphQLClientNoPool(p.apiUrl, p.subUrl, p.config)

	if deviceID, ok = p.config.DeviceContext().GetDeviceID(); !ok {
		return nil, fmt.Errorf("unable to determine current client's device context")
	}
	sourceUrn.WriteString("urn:mycs:device:")
	sourceUrn.WriteString(deviceID)
	sourceUrn.WriteByte(':')
	sourceUrn.WriteString(p.config.DeviceContext().GetLoggedInUserID())
	eventSource = sourceUrn.String()

	var mutation struct {
		PublishData []events.PublishEventResult `graphql:"publishData(data: $data)"`
	}
	variables := map[string]interface{}{
		"data": events.CreatePublishEventList(eventSource, cloudEvents),
	}
	if err = apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("EventsAPI.PostMeasurementEvents(): publishData mutation returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("EventsAPI.PostMeasurementEvents(): publishData mutation returned response: %# v", mutation)

	return events.CreateCloudEventErrorList(mutation.PublishData, cloudEvents), nil
}

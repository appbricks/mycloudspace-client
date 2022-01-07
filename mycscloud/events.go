package mycscloud

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/hasura/go-graphql-client"
	"github.com/sirupsen/logrus"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-common/monitors"
	"github.com/mevansam/goutils/logger"
)

type EventPublisher struct {
	config config.Config
	apiUrl, 
	subUrl string
}

type PublishDataInput struct {
	Type       graphql.String  `json:"type"`
	Compressed graphql.Boolean `json:"compressed"`
	Payload    graphql.String  `json:"payload"`
}

func NewEventPublisher(apiUrl, subUrl string, config config.Config) *EventPublisher {

	return &EventPublisher{
		config: config,
		apiUrl: apiUrl,
		subUrl: subUrl,
	}
}

func (p *EventPublisher) PostMeasurementEvents(events []*cloudevents.Event) ([]monitors.PostEventErrors, error) {

	var (
		err error
		ok  bool

		deviceID,
		eventSource string
		sourceUrn strings.Builder

		zlibWriter *zlib.Writer

		eventPayload      []byte
		compressedPayload bytes.Buffer
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
		PublishData []struct {
			Success graphql.Boolean
			Error   graphql.String
		} `graphql:"publishData(data: $data)"`
	}

	dataPayloads := make([]PublishDataInput, 0, len(events))
	for _, event := range events {
		event.SetSource(eventSource)

		if logrus.IsLevelEnabled(logrus.TraceLevel) {
			logger.DebugMessage("EventsAPI.PostMeasurementEvents(): Preparing device event for posting: %s", event.String())
		}
		
		if eventPayload, err = event.MarshalJSON(); err != nil {
			logger.ErrorMessage("EventsAPI.PostMeasurementEvents(): Unable to marshal event: %s", err.Error())
			continue
		}

		// compress payload and add it to list of payloads
		compressedPayload.Reset()
		zlibWriter = zlib.NewWriter(&compressedPayload)
		if _, err = zlibWriter.Write([]byte(eventPayload)); err != nil {
			logger.ErrorMessage("EventsAPI.PostMeasurementEvents(): Unable to marshal event: %s", event.String())
			zlibWriter.Close()
			continue
		}
		zlibWriter.Close()

		dataPayloads = append(dataPayloads, PublishDataInput{
			Type: "event",
			Compressed: graphql.Boolean(true),
			Payload: graphql.String(base64.StdEncoding.EncodeToString(compressedPayload.Bytes())),
		})
	}

	variables := map[string]interface{}{
		"data": dataPayloads,
	}
	if err := apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("EventsAPI.PostMeasurementEvents(): publishData mutation returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("EventsAPI.PostMeasurementEvents(): publishData mutation returned response: %# v", mutation)

	errors := []monitors.PostEventErrors{}
	for i, result := range mutation.PublishData {
		if !bool(result.Success) {
			errors = append(errors, monitors.PostEventErrors{
				Event: events[i],
				Error: string(result.Error),
			})
		}
	}
	return errors, nil
}

package api

import (
	"net"
	"net/http"
	"time"

	graphql "github.com/hasura/go-graphql-client"

	"github.com/appbricks/cloud-builder/config"
)

// returns a graphql client for querying
// the MyCS cloud API service
func NewGraphQLClient(apiUrl, subUrl string, config config.Config) *graphql.Client {

	return graphql.NewClient(apiUrl, &http.Client{
		Transport: authHeader{
			idToken:   config.AuthContext().GetToken().Extra("id_token").(string),
			transport: http.DefaultTransport,
		},
	})
}

// returns a graphql client for querying
// the MyCS cloud API service which is not
// pooled for reuse and has a very short
// timeout
func NewGraphQLClientNoPool(apiUrl, subUrl string, config config.Config) *graphql.Client {

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	transport.MaxIdleConnsPerHost = -1
	transport.TLSHandshakeTimeout = 1000 * time.Millisecond
	transport.DialContext = (&net.Dialer{ Timeout: 1000 * time.Millisecond }).DialContext
	
	return graphql.NewClient(apiUrl, &http.Client{
		Transport: authHeader{
			idToken:   config.AuthContext().GetToken().Extra("id_token").(string),
			transport: transport,
		},
		Timeout: 5000 * time.Millisecond,
	})
}

// AWS Cognito authorization token
type authHeader struct {
	idToken   string
	transport http.RoundTripper
}

// adds the Cognito authorization id 
// token to all client query requests
func (h authHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Authorization", h.idToken)
	return h.transport.RoundTrip(req)
}

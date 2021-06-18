package api

import (
	"net/http"

	graphql "github.com/hasura/go-graphql-client"

	"github.com/appbricks/cloud-builder/config"
)

// returns a graphql client for querying
// the MyCS cloud API service
func NewGraphQLClient(apiUrl, subUrl string, config config.Config) *graphql.Client {

	return graphql.NewClient(apiUrl, &http.Client{
		Transport: authHeader{
			idToken: config.AuthContext().GetToken().Extra("id_token").(string),
		},
	})
}

// AWS Cognito authorization token
type authHeader struct {
	idToken string
}

// adds the Cognito authorization id 
// token to all client query requests
func (h authHeader) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Accept-Encoding", "gzip, deflate, br")
	req.Header.Add("Authorization", h.idToken)
	return http.DefaultTransport.RoundTrip(req)
}

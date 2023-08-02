package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/mevansam/goutils/logger"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/mycloudspace-client/api"
)

type AWSCognitoJWT struct {
	jwkSet    jwk.Set
	jwtToken  jwt.Token
}

type Preferences struct {
	PreferredName   string `json:"preferredName,omitempty"`
	EnableBiometric bool   `json:"enableBiometric"`
	EnableMFA       bool   `json:"enableMFA"`
	EnableTOTP      bool   `json:"enableTOTP"`
	RememberFor24h  bool   `json:"rememberFor24h"`
}

func NewAWSCognitoJWT(serviceConfig api.ServiceConfig, authContext config.AuthContext) (*AWSCognitoJWT, error) {

	var (
		err error
	)
	awsJWT := &AWSCognitoJWT{}

	if awsJWT.jwkSet, err = jwk.Fetch(
		context.Background(), 
		fmt.Sprintf(
			"https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", 
			serviceConfig.Region, 
			serviceConfig.UserPoolID,
		),
	); err != nil {
		return nil, err
	}
	if awsJWT.jwtToken, err = jwt.Parse(
		[]byte(authContext.GetToken().Extra("id_token").(string)),
		jwt.WithKeySet(awsJWT.jwkSet),
	); err != nil {
		return nil, err
	}
	logger.TraceMessage("JWT Token for logged in user is: %# v", awsJWT.jwtToken)

	return awsJWT, nil
}

func (awsJWT *AWSCognitoJWT) UserID() string {
	username, _ := awsJWT.jwtToken.Get("custom:userID")
	return username.(string)
}

func (awsJWT *AWSCognitoJWT) Username() string {
	username, _ := awsJWT.jwtToken.Get("cognito:username")
	return username.(string)
}

func (awsJWT *AWSCognitoJWT) Preferences() *Preferences {

	var (
		err error
		ok  bool

		value   interface{}
		data    string
	)
	prefs := &Preferences{}
	
	if value, _ = awsJWT.jwtToken.Get("custom:preferences"); value == nil {
		return prefs
	}
	if data, ok = value.(string); !ok {
		logger.ErrorMessage(
			"JWT Token claim custom:preferences is not the expected type: %# v", 
			value,
		)
		return prefs
	}
	if err = json.Unmarshal([]byte(data), prefs); err != nil {
		logger.ErrorMessage(
			"Unable to parse JWT Token claim custom:preferences with value '%s': %s", 
			data, err.Error(),
		)
	}
	return prefs
}

func (awsJWT *AWSCognitoJWT) KeyTimestamp() int64 {
	return awsJWT.getTimesampClaim("custom:keyTimestamp")
}

func (awsJWT *AWSCognitoJWT) ConfigTimestamp() int64 {
	return awsJWT.getTimesampClaim("custom:configTimestamp")
}

func (awsJWT *AWSCognitoJWT) getTimesampClaim(claimKey string) int64 {

	var (
		err error
		ok  bool

		value   interface{}
		ts      string
		tsValue int64
	)
	
	if value, _ = awsJWT.jwtToken.Get(claimKey); value == nil {
		return 0
	}	
	if ts, ok = value.(string); !ok {
		logger.ErrorMessage(
			"JWT Token claim %s is not the expected type: %# v", 
			claimKey, value,
		)
		return 0
	}
	if tsValue, err = strconv.ParseInt(ts, 10, 64); err != nil {
		logger.ErrorMessage(
			"Unable to parse JWT Token claim %s with value '%s': %s", 
			claimKey, ts, err.Error(),
		)
		return 0
	}
	return tsValue
}

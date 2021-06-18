package auth

import (
	"context"
	"fmt"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/mevansam/goutils/logger"

	"github.com/appbricks/cloud-builder/config"
)

const AWS_COGNITO_REGION = `us-east-1`
const AWS_COGNITO_USER_POOL_ID = `us-east-1_hyOWP6bHf`

type AWSCognitoJWT struct {
	jwkSet    jwk.Set
	jwtToken  jwt.Token
}

func NewAWSCognitoJWT(config config.Config) (*AWSCognitoJWT, error) {

	var (
		err error
	)
	awsJWT := &AWSCognitoJWT{}

	if awsJWT.jwkSet, err = jwk.Fetch(
		context.Background(), 
		fmt.Sprintf(
			"https://cognito-idp.%s.amazonaws.com/%s/.well-known/jwks.json", 
			AWS_COGNITO_REGION, 
			AWS_COGNITO_USER_POOL_ID,
		),
	); err != nil {
		return nil, err
	}

	token := config.AuthContext().GetToken()
	if awsJWT.jwtToken, err = jwt.Parse(
		[]byte(token.Extra("id_token").(string)),
		jwt.WithKeySet(awsJWT.jwkSet),
	); err != nil {
		return nil, err
	}
	logger.TraceMessage("JWT Token for logged in user is: %# v", awsJWT.jwtToken)

	return awsJWT, nil
}

func (awsJWT *AWSCognitoJWT) Username() string {
	username, _ := awsJWT.jwtToken.Get("cognito:username")
	return username.(string)
}

package api

// AWS API service configuration
type ServiceConfig struct {

	// AWS Region
	Region string

	// Cognito user pool ID
	UserPoolID string

	// User pool resource app 
	// client ID and secret
	CliendID, 
	ClientSecret string

	// Endpoint URLs
	AuthURL, 
	TokenURL,
	ApiURL string
}

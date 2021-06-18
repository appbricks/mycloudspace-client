package mycscloud

import "github.com/hasura/go-graphql-client"

type appAPI struct {
	apiClient *graphql.Client
}

func NewAppAPI(apiClient *graphql.Client) *appAPI {

	return &appAPI{
		apiClient: apiClient,
	}
}

func (s *appAPI) CreateSpace() {

}

func (s *appAPI) DeleteSpace() {

}

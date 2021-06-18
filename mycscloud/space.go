package mycscloud

import "github.com/hasura/go-graphql-client"

type spaceAPI struct {
	apiClient *graphql.Client
}

func NewSpaceAPI(apiClient *graphql.Client) *spaceAPI {

	return &spaceAPI{
		apiClient: apiClient,
	}
}

func (s *spaceAPI) CreateSpace() {

}

func (s *spaceAPI) DeleteSpace() {

}

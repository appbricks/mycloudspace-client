package mycscloud

import (
	"context"
	"fmt"

	"github.com/hasura/go-graphql-client"

	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/mevansam/goutils/logger"
)

type DeviceAPI struct {
	apiClient *graphql.Client
}

func NewDeviceAPI(apiClient *graphql.Client) *DeviceAPI {
	return &DeviceAPI{
		apiClient: apiClient,
	}
}

func (d *DeviceAPI) UpdateDeviceContext(deviceContext config.DeviceContext) error {
	
	var (
		deviceID, ownerUserID string
		exists                bool

		guestUser *userspace.User
		userID, userName, status string
	)

	if deviceID, exists = deviceContext.GetDeviceID(); !exists {
		return fmt.Errorf("device context has not been initialized with a device")
	}
	if ownerUserID, exists = deviceContext.GetOwnerUserID(); !exists {
		return fmt.Errorf("device context has not been initialized with an owner")
	}	

	var query struct {
		AuthDevice struct {
			AccessType graphql.String
			Device struct {
				DeviceID graphql.String `graphql:"deviceID"`
				DeviceName graphql.String
				Users struct {
					DeviceUsers []struct {
						User struct {
							UserID graphql.String `graphql:"userID"`
							UserName graphql.String	
						}
						IsOwner graphql.Boolean	
						Status graphql.String	
					}
				}	
			}
		} `graphql:"authDevice(idKey: $idKey)"`
	}

	variables := map[string]interface{}{
		"idKey": graphql.String(deviceContext.GetDeviceIDKey()),
	}
	if err := d.apiClient.Query(context.Background(), &query, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.UpdateDeviceContext(): authDevice query returned an error: %s", err.Error())
		return err
	}
	logger.TraceMessage("DeviceAPI.UpdateDeviceContext(): authDevice query returned response: %# v", query)

	if string(query.AuthDevice.AccessType) == "admin" {
		// check if logged in user is the admin
		userID, _ = deviceContext.GetOwnerUserID()
		if deviceContext.GetLoggedInUserID() != userID {
			logger.ErrorMessage(
				"DeviceAPI.UpdateDeviceContext(): authDevice query returned \"admin\" access type for a user that is not the device owner.",
			)
			return fmt.Errorf("invalid device context")
		}

		// check if authorized device matches device in context
		if string(query.AuthDevice.Device.DeviceID) != deviceID {
			logger.ErrorMessage(
				"DeviceAPI.UpdateDeviceContext(): authDevice query returned device ID '%s' but the device context device id was '%s'.",
				query.AuthDevice.Device.DeviceID, deviceID,
			)
			return fmt.Errorf("invalid device context")
		}

		device := deviceContext.GetDevice()
		guestUsers := deviceContext.ResetGuestUsers()

		device.Name = string(query.AuthDevice.Device.DeviceName)
		for _, deviceUser := range query.AuthDevice.Device.Users.DeviceUsers {
			userID = string(deviceUser.User.UserID)
			userName = string(deviceUser.User.UserName)
			status = string(deviceUser.Status)
			if bool(deviceUser.IsOwner) {
				// validate owner
				if ownerUserID != userID {
					logger.ErrorMessage(
						"DeviceAPI.UpdateDeviceContext(): authDevice query returned owner user ID '%s' but the device context owner has user id '%s'.",
						deviceUser.User.UserID, ownerUserID,
					)
					return fmt.Errorf("invalid device context")
				}
			} else {
				if guestUser, exists = guestUsers[userName]; exists && guestUser.UserID ==userID {
					guestUser.Active = (status == "active")
					deviceContext.AddGuestUser(guestUser)
				} else {
					logger.WarnMessage(
						"DeviceAPI.UpdateDeviceContext(): authDevice query returned guest user ID '%s' that was not present in the device context.",
						deviceUser.User.UserID,
					)
				}
			}
		}
	} else {
		if string(query.AuthDevice.AccessType) == "unauthorized" {
			return fmt.Errorf("unauthorized")
		}
		if guestUser, exists = deviceContext.GetGuestUser(deviceContext.GetLoggedInUserName()); !exists {
			logger.ErrorMessage(
				"DeviceAPI.UpdateDeviceContext(): authDevice query returned a guest user \"%s\" that was not found in the device context",
				guestUser.UserID,
			)
			return fmt.Errorf("invalid device context")
		}
		guestUser.Active = string(query.AuthDevice.AccessType) == "guest"
		if !guestUser.Active {
			return fmt.Errorf("unauthorized")
		}
	}
	return nil
}

func (d *DeviceAPI) RegisterDevice(
	deviceName, 
	deviceType,
	clientVersion,
	deviceCertRequest,
	devicePublicKey,
	wireguardPublicKey string,
) (string, string, error) {

	var mutation struct {
		AddDevice struct {
			IdKey graphql.String
			DeviceUser struct {
				Device struct {
					DeviceID graphql.String `graphql:"deviceID"`
				}
			}
		} `graphql:"addDevice(deviceName: $deviceName, deviceInfo: { deviceType: $deviceType, clientVersion: $clientVersion }, deviceKey: {publicKey: $devicePublicKey, certificateRequest: $deviceCertRequest}, accessKey: {wireguardPublicKey: $wireguardPublicKey})"`
	}
	variables := map[string]interface{}{
		"deviceName": graphql.String(deviceName),
		"deviceType": graphql.String(deviceType),
		"clientVersion": graphql.String(clientVersion),
		"deviceCertRequest": graphql.String(deviceCertRequest),
		"devicePublicKey": graphql.String(devicePublicKey),
		"wireguardPublicKey": graphql.String(wireguardPublicKey),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.RegisterDevice(): addDevice mutation returned an error: %s", err.Error())
		return "", "", err
	}
	logger.TraceMessage("DeviceAPI.RegisterDevice(): addDevice mutation returned response: %# v", mutation)
	return string(mutation.AddDevice.IdKey), string(mutation.AddDevice.DeviceUser.Device.DeviceID), nil
}

func (d *DeviceAPI) UnRegisterDevice(deviceID string) ([]string, error) {

	var mutation struct {
		DeleteDevice []string `graphql:"deleteDevice(deviceID: $deviceID)"`
	}
	variables := map[string]interface{}{
		"deviceID": graphql.ID(deviceID),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.UnRegisterDevice(): deleteDevice mutation returned an error: %s", err.Error())
		return nil, err
	}
	logger.TraceMessage("DeviceAPI.UnRegisterDevice(): deleteDevice mutation returned response: %# v", mutation)

	userIDs := []string{}
	for _, userID := range mutation.DeleteDevice {
		userIDs = append(userIDs, string(userID))
	}
	return userIDs, nil
}

func (d *DeviceAPI) AddDeviceUser(deviceID, wireguardPublicKey string) (string, string, error) {

	var mutation struct {
		AddDeviceUser struct {
			Device struct {
				DeviceID graphql.String `graphql:"deviceID"`
			}
			User struct {
				UserID graphql.String `graphql:"userID"`
			}
		} `graphql:"addDeviceUser(deviceID: $deviceID, accessKey: {wireguardPublicKey: $wireguardPublicKey})"`
	}
	variables := map[string]interface{}{
		"deviceID": graphql.ID(deviceID),
		"wireguardPublicKey": graphql.String(wireguardPublicKey),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.AddDeviceUser(): addDeviceUser mutation returned an error: %s", err.Error())
		return "", "", err
	}
	logger.TraceMessage("DeviceAPI.AddDeviceUser(): addDeviceUser mutation returned response: %# v", mutation)
	return string(mutation.AddDeviceUser.Device.DeviceID), string(mutation.AddDeviceUser.User.UserID), nil
}

func (d *DeviceAPI) RemoveDeviceUser(deviceID string) (string, string, error) {

	var mutation struct {
		DeleteDeviceUser struct {
			Device struct {
				DeviceID graphql.String `graphql:"deviceID"`
			}
			User struct {
				UserID graphql.String `graphql:"userID"`
			}
		} `graphql:"deleteDeviceUser(deviceID: $deviceID)"`
	}
	variables := map[string]interface{}{
		"deviceID": graphql.ID(deviceID),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.RemoveDeviceUser(): deleteDeviceUser mutation returned an error: %s", err.Error())
		return "", "", err
	}
	logger.TraceMessage("DeviceAPI.RemoveDeviceUser(): deleteDeviceUser mutation returned response: %# v", mutation)
	return string(mutation.DeleteDeviceUser.Device.DeviceID), string(mutation.DeleteDeviceUser.User.UserID), nil
}

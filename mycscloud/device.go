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
				DeviceType graphql.String
				ManagedDevices []struct {
					DeviceID graphql.String `graphql:"deviceID"`
					Users struct {
						DeviceUsers []struct {
							User struct {
								UserID graphql.String `graphql:"userID"`
								UserName graphql.String
								FirstName graphql.String
								MiddleName graphql.String
								FamilyName graphql.String
							}
						}
					}	
				}
				Users struct {
					DeviceUsers []struct {
						User struct {
							UserID graphql.String `graphql:"userID"`
							UserName graphql.String	
							FirstName graphql.String
							MiddleName graphql.String
							FamilyName graphql.String
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
	logger.DebugMessage("DeviceAPI.UpdateDeviceContext(): authDevice query returned response: %# v", query)

	if string(query.AuthDevice.AccessType) == "admin" {
		// check if logged in user is the admin
		if deviceContext.GetLoggedInUserID() != ownerUserID {
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

		// update managed devices in context
		managedDevicesInContext := make(map[string]*userspace.Device)
		for _, d := range deviceContext.GetManagedDevices() {
			managedDevicesInContext[d.DeviceID] = d
		}
		for _, d := range query.AuthDevice.Device.ManagedDevices {
			deviceID = string(d.DeviceID)
			if md := managedDevicesInContext[deviceID]; md != nil {
				for _, du := range d.Users.DeviceUsers {
					userID = string(du.User.UserID)
					if userID != ownerUserID {
						md.DeviceUsers = append(md.DeviceUsers, 
							&userspace.User{
								UserID: userID,
								Name: string(du.User.UserName),
								FirstName: string(du.User.FirstName),
								MiddleName: string(du.User.MiddleName),
								FamilyName: string(du.User.FamilyName),
							},
						)	
					}
				}
				delete(managedDevicesInContext, deviceID)
			}
		}
		for deviceID = range managedDevicesInContext {
			deviceContext.DeleteManageDevice(deviceID)
		}

		// updated device users
		device.Name = string(query.AuthDevice.Device.DeviceName)
		device.Type = string(query.AuthDevice.Device.DeviceType)
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
					guestUser.FirstName = string(deviceUser.User.FirstName)
					guestUser.MiddleName = string(deviceUser.User.MiddleName)
					guestUser.FamilyName = string(deviceUser.User.FamilyName)
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
			return fmt.Errorf("unauthorized(pending)")
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
	managedBy string,
) (string, string, error) {

	var mutation struct {
		AddDevice struct {
			IdKey graphql.String
			DeviceUser struct {
				Device struct {
					DeviceID graphql.String `graphql:"deviceID"`
				}
			}
		} `graphql:"addDevice(deviceName: $deviceName, deviceInfo: { deviceType: $deviceType, clientVersion: $clientVersion, managedBy: $managedBy }, deviceKey: {publicKey: $devicePublicKey, certificateRequest: $deviceCertRequest})"`
	}
	variables := map[string]interface{}{
		"deviceName": graphql.String(deviceName),
		"deviceType": graphql.String(deviceType),
		"clientVersion": graphql.String(clientVersion),
		"deviceCertRequest": graphql.String(deviceCertRequest),
		"devicePublicKey": graphql.String(devicePublicKey),
		"managedBy": graphql.String(managedBy),
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

func (d *DeviceAPI) AddDeviceUser(deviceID, userID string) (string, string, error) {

	var mutation struct {
		AddDeviceUser struct {
			Device struct {
				DeviceID graphql.String `graphql:"deviceID"`
			}
			User struct {
				UserID graphql.String `graphql:"userID"`
			}
		} `graphql:"addDeviceUser(deviceID: $deviceID, userID: $userID)"`
	}
	variables := map[string]interface{}{
		"deviceID": graphql.ID(deviceID),
		"userID": graphql.ID(userID),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.AddDeviceUser(): addDeviceUser mutation returned an error: %s", err.Error())
		return "", "", err
	}
	logger.TraceMessage("DeviceAPI.AddDeviceUser(): addDeviceUser mutation returned response: %# v", mutation)
	return string(mutation.AddDeviceUser.Device.DeviceID), string(mutation.AddDeviceUser.User.UserID), nil
}

func (d *DeviceAPI) RemoveDeviceUser(deviceID, userID string) (string, string, error) {

	var mutation struct {
		DeleteDeviceUser struct {
			Device struct {
				DeviceID graphql.String `graphql:"deviceID"`
			}
			User struct {
				UserID graphql.String `graphql:"userID"`
			}
		} `graphql:"deleteDeviceUser(deviceID: $deviceID, userID: $userID)"`
	}
	variables := map[string]interface{}{
		"deviceID": graphql.ID(deviceID),
		"userID": graphql.ID(userID),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.RemoveDeviceUser(): deleteDeviceUser mutation returned an error: %s", err.Error())
		return "", "", err
	}
	logger.TraceMessage("DeviceAPI.RemoveDeviceUser(): deleteDeviceUser mutation returned response: %# v", mutation)
	return string(mutation.DeleteDeviceUser.Device.DeviceID), string(mutation.DeleteDeviceUser.User.UserID), nil
}

func (d *DeviceAPI) SetDeviceWireguardConfig(
	userID,
	deviceID,
	spaceID,
	wgConfigName,
	wgConfig string,
	wgExpirationTimeout,
	wgInactivityTimeout int,
) error {

	var mutation struct {
		UpdateDevice struct {
			WGConfigName graphql.String `graphql:"wgConfigName"`
		} `graphql:"setDeviceUserSpaceConfig(userID: $userID, deviceID: $deviceID, spaceID: $spaceID, config: { wgConfigName: $wgConfigName, wgConfig: $wgConfig, wgExpirationTimeout: $wgExpirationTimeout, wgInactivityTimeout: $wgInactivityTimeout})"`
	}
	variables := map[string]interface{}{
		"userID": graphql.ID(userID),
		"deviceID": graphql.ID(deviceID),
		"spaceID": graphql.ID(spaceID),
		"wgConfigName": graphql.String(wgConfigName),
		"wgConfig": graphql.String(wgConfig),
		"wgExpirationTimeout": graphql.Int(wgExpirationTimeout),
		"wgInactivityTimeout": graphql.Int(wgInactivityTimeout),
	}
	if err := d.apiClient.Mutate(context.Background(), &mutation, variables); err != nil {
		logger.ErrorMessage("DeviceAPI.SetDeviceWireguardConfig(): setDeviceUserSpaceConfig mutation returned an error: %s", err.Error())
		return err
	}
	logger.TraceMessage("DeviceAPI.SetDeviceWireguardConfig(): setDeviceUserSpaceConfig mutation returned response: %# v", mutation)
	return nil
}

package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/appbricks/cloud-builder/config"
	cb_config "github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/auth"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/system"
	"github.com/appbricks/mycloudspace-client/ui"
	"github.com/hasura/go-graphql-client"
	"github.com/mevansam/goutils/crypto"
	"github.com/mevansam/goutils/logger"
)

type ConfigInitializer struct {
	ctx context.Context

	appConfig cb_config.Config

	// flags that the config is being
	// is being reset to a new owner
	resetConfig bool

	// api client and device id of current owner. 
	// required for current owner/device if reset
	currOwnerAPIClient *graphql.Client 
	currOwnerUserID    string
	currDeviceID       string

	// new owner/device details
	newOwnerAPIClient *graphql.Client

	updateUserKey bool

	// mycs service configuration
	serviceConfig api.ServiceConfig
	// user interaction abstraction
	appUI ui.UI
}

type KeyRet struct {
	Key   *crypto.RSAKey
	Error error
}
type AsyncKeyRet chan KeyRet

func NewConfigInitializer(
	forAppConfig config.Config,
	ctx context.Context,
	serviceConfig api.ServiceConfig,
	appUI ui.UI,
) (*ConfigInitializer, error) {

	var (
		err error

		ownerOK, deviceOK bool

		appConfig cb_config.Config
	)

	logger.DebugMessage(
		"Loading application configuration at '%s' for initialization/updates", 
		forAppConfig.GetConfigFile(),
	)

	if appConfig, err = cb_config.InitFileConfig(
		forAppConfig.GetConfigFile(), nil, 
		func() string { 
			return forAppConfig.GetPassphrase()
		}, nil,
	); err != nil {
		return nil, err
	}
	if err = appConfig.Load(); err != nil {
		return nil, err
	}

	ci := &ConfigInitializer{
		ctx: ctx,
		
		appConfig:  appConfig,

		serviceConfig: serviceConfig,
		appUI:				 appUI,
	}
	if ci.appConfig.Initialized() {
		// set/validate current configuration owner/device details
		deviceContext := appConfig.DeviceContext()
		ci.currOwnerUserID, ownerOK = deviceContext.GetOwnerUserID()
		ci.currDeviceID, deviceOK = deviceContext.GetDeviceID()		
		if !ownerOK || !deviceOK {
			return nil, fmt.Errorf("device configuration is in an inconsistent state")
		}
		ci.currOwnerAPIClient = api.NewGraphQLClient(serviceConfig.ApiURL, "", appConfig.AuthContext())
		ci.resetConfig = false
		
	} else {
		ci.resetConfig = true
	}
	return ci, nil
}

func (ci *ConfigInitializer) AppConfig() cb_config.Config {
	return ci.appConfig
}

func (ci *ConfigInitializer) Initialized() bool {
	return ci.appConfig.Initialized()
}

func (ci *ConfigInitializer) DeviceUsername() string {
	ownerUserName, _ := ci.appConfig.DeviceContext().GetOwnerUserName()
	return ownerUserName
}

func (ci *ConfigInitializer) DeviceName() string {
	deviceName, _ := ci.appConfig.DeviceContext().GetDeviceName()
	return deviceName
}

func (ci *ConfigInitializer) DevicePassphrase() string {
	return ci.appConfig.GetPassphrase()
}

func (ci *ConfigInitializer) UnlockedTimeout() int {
	return int(ci.appConfig.GetKeyTimeout() / time.Minute)
}

func (ci *ConfigInitializer) ResetDeviceOwner(
	handleAuthResult func(userName, deviceName string, userNeedsNewKey bool, err error),
) {
	
	go func() {

		var (
			err error			

			tokenRet auth.AsyncTokenRet
			token    auth.TokenRet

			newAppConfig cb_config.Config

			newUserName, 
			newDeviceName   string
			userNeedsNewKey bool
		)

		// always call handler on exit
		defer func() {
			if tokenRet != nil {
				close(tokenRet)
			}
			if err != nil {
				ci.newOwnerAPIClient = nil
				ci.resetConfig = !ci.appConfig.Initialized()				
			} else {
				ci.appConfig = newAppConfig
			}
			handleAuthResult(
				newUserName, 
				newDeviceName,
				userNeedsNewKey, 
				err,
			)
		}()

		// create a new config instance
		if newAppConfig, err = cb_config.InitFileConfig(
			ci.appConfig.GetConfigFile(), nil, 
			func() string { 
				return ""
			}, nil,
		); err != nil {
			return
		}

		deviceOwner, ownerIsSet := ci.appConfig.DeviceContext().GetOwnerUserName()
		if ci.appConfig.Initialized() && ownerIsSet {
			// confirm device owner by forcing user to re-login
			confirmInput := make(chan bool, 1)
			defer close(confirmInput)

			uh := ci.appUI.NewUIMessage("Confirm Reset Owner")
			uh.WriteDangerMessage(
				"Resetting the user that is registered as the owner of this device will remove " + 
				"the device from the his/her device list and reset any saved configurations. " + 
				"Do you wish to continue?",
			)
			uh.ShowMessageWithYesNoInput(func(confirm bool) {
				confirmInput <-confirm
			})
			if !<-confirmInput {
				err = fmt.Errorf("device owner reset cancelled")
				return
			}
			tokenRet = auth.GetAuthenticatedToken(ci.ctx, ci.serviceConfig, newAppConfig, true, ci.appUI,
				"To continue please authenticate as the current device owner whose configuration will be overwritten.",
			)

			token = <-tokenRet
			if err = token.Error; err != nil {				
				return
			}

			// ensure device reset is done only by the device owner
			if token.AWSAuth.Username() != deviceOwner {
				ci.appUI.ShowErrorMessage("In order to re-initialize a configuration you need to sign in as the current device owner.")
				err = fmt.Errorf("device owner not logged in")
				return
			}
			
			close(tokenRet)
		}

		// login new device owner
		tokenRet = auth.GetAuthenticatedToken(ci.ctx, ci.serviceConfig, newAppConfig, true, ci.appUI,
			"Please login as the primary user that will be configured as the owner of this device.",
		)
		token = <-tokenRet
		if err = token.Error; err != nil {				
			return
		}

		if deviceOwner == token.AWSAuth.Username() {
			ci.appUI.ShowInfoMessage("Device Owner Reset", fmt.Sprintf("User \"%s\" is already the device owner.", deviceOwner))
			err = fmt.Errorf("device owner already set")
			return
		}

		if newUserName, newDeviceName, err = ci.resetDeviceOwner(token.AWSAuth, newAppConfig); err != nil {
			ci.appUI.ShowErrorMessage(err.Error())
			return
		}

		userNeedsNewKey = token.AWSAuth.KeyTimestamp() == 0
	}()
}

func (ci *ConfigInitializer) resetDeviceOwner(
	awsAuth *auth.AWSCognitoJWT,
	newAppConfig cb_config.Config,
) (string, string, error) {

	var (
		err error

		userID string
		owner  *userspace.User
		device *userspace.Device

		newUserName,
		newDeviceName string
	)
	
	deviceContext := newAppConfig.DeviceContext()
	newUserName = awsAuth.Username()
	userID = awsAuth.UserID()

	// create device owner user
	if owner, err = deviceContext.NewOwnerUser(userID, newUserName); err != nil {
		return "", "", fmt.Errorf("failed to initialize new device owner: %s", err.Error())	
	}
	// retrieve the owner user details
	ci.newOwnerAPIClient = api.NewGraphQLClient(ci.serviceConfig.ApiURL, "", newAppConfig.AuthContext())
	userAPI := mycscloud.NewUserAPI(ci.newOwnerAPIClient)
	if _, err = userAPI.GetUser(owner); err != nil {
		return "", "", fmt.Errorf("failed to retrieve user '%s' from MyCS cloud: %s", owner.Name, err.Error())
	}
	// create new device to associate with the owner
	if device, err = deviceContext.NewDevice(); err != nil {
		return "", "", fmt.Errorf("failed to initiaize new device for user '%s': %s", owner.Name, err.Error())	
	}
	if device.Name == "" {
		newDeviceName = owner.Name + "-device"
	} else {
		newDeviceName = device.Name
	}
	ci.resetConfig = true
	
	return newUserName, newDeviceName, nil
}

func (ci *ConfigInitializer) LoadDeviceOwnerKey(
	keyFileURL string,
	createKey bool,
	handleKeyLoadedResult func(keyFileName string, err error),
) {

	keyFileName := strings.TrimPrefix(keyFileURL, "file://")
	keyRet := ci.getPrivateKey(keyFileName, createKey, ci.appUI)

	go func() {

		var (
			err error

			owner *userspace.User
		)

		defer close(keyRet)
		key := <-keyRet

		// always call handler on exit
		defer func() {
			if err != nil {
				if createKey {
					ci.appUI.ShowErrorMessage(
						fmt.Sprintf(
							"Unable to create/update key file: %s", 
							err.Error(),
						),
					)
				} else {
					ci.appUI.ShowErrorMessage(
						fmt.Sprintf(
							"Unable to open key file: %s", 
							err.Error(),
						),
					)
				}
			}
			handleKeyLoadedResult(keyFileName, err)
		}()

		if err = key.Error; err == nil {

			if key.Key != nil {
				if owner = ci.appConfig.DeviceContext().GetOwner(); owner == nil {
					err = fmt.Errorf("cannot set key as device owner not configured")

				} else if err = owner.SetKey(key.Key, createKey); err != nil {
					err = fmt.Errorf("incorrect key for user '%s'.", owner.Name)

				} else {
					ci.updateUserKey = true
				}

			} else {
				err = fmt.Errorf("key file not loaded and no error was returned")
			}
		}
	}()
}

func (ci *ConfigInitializer) getPrivateKey(
	keyFileName string, 
	createKey bool, 
	ui ui.UI,
) AsyncKeyRet {

	var (
		err error

		keyFileExists bool

		key        *crypto.RSAKey
		keyFilePEM string
	)

	keyRet := make(AsyncKeyRet, 1)

	keyFileExists = true
	if _, err = os.Stat(keyFileName); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			keyFileExists = false
		} else {
			keyRet <-KeyRet{nil, err}
			return keyRet
		}
	}

	if createKey {
		if !keyFileExists {
			uh := ui.NewUIMessage("Key File Passphrase")
			uh.WriteInfoMessage(
				"Please enter and verify the key file passphrase. " + 
				"This will be used to encrypt the private key.",
			)
			uh.ShowMessageWithSecureVerifiedInput(func(keyFilePassphrase *string) {
				if key, err = crypto.NewRSAKey(); err != nil {
					keyRet <-KeyRet{nil, err}
				} else {
					if keyFilePEM, err = key.GetEncryptedPrivateKeyPEM([]byte(*keyFilePassphrase)); err != nil {
						keyRet <-KeyRet{nil, err}
					} else {
						if err = os.WriteFile(keyFileName, []byte(keyFilePEM), 0600); err != nil {
							keyRet <-KeyRet{nil, err}
						} else {
							ui.ShowInfoMessage(
								"Key File Created", 
								"This key will be used to secure all data associated with the device user " +
								"as well as establishing a verifiable identity. It is important that this " +
								"key is saved in a secure location offline such as a USB key which can be " +
								"locked away in a safe.",
							)
							keyRet <-KeyRet{key, nil}
						}
					}
				}
			})
		} else {
			keyRet <-KeyRet{nil, fmt.Errorf("a file already exists at the given path '%s'", keyFileName)}
		}
		
	} else if keyFileExists {
		uh := ui.NewUIMessage("Key File Passphrase")
		uh.WriteInfoMessage("Enter the passphrase needed to open the key file.")
		uh.ShowMessageWithSecureInput(func(keyFilePassphrase *string) {
			if key, err = crypto.NewRSAKeyFromFile(keyFileName, []byte(*keyFilePassphrase)); err != nil {
				keyRet <-KeyRet{nil, err}
			} else {
				keyRet <-KeyRet{key, nil}
			}
		})
	} else {
		keyRet <-KeyRet{nil, fmt.Errorf("key file '%s' does not exist", keyFileName)}
	}
	return keyRet
}

func (ci *ConfigInitializer) Save(
	deviceName, 
	deviceLockPassphrase,
	clientType,
	clientVersion string, 
	unlockedTimeout int,
	handleSaveResult func(err error),
) {

	go func() {

		var (
			err error
	
			userAPI   *mycscloud.UserAPI
			deviceAPI *mycscloud.DeviceAPI
	
			deviceIDKey,
			deviceID string
		)

		// always call handler on exit
		defer func() {
			if err != nil {
				ci.appUI.ShowErrorMessage(
					fmt.Sprintf(
						"Unable to save settings: %s", 
						err.Error(),
					),
				)
			}
			handleSaveResult(err)
		}()

		deviceContext := ci.appConfig.DeviceContext()

		if ci.updateUserKey {
			if ci.newOwnerAPIClient != nil {
				userAPI = mycscloud.NewUserAPI(ci.newOwnerAPIClient)
			} else {
				userAPI = mycscloud.NewUserAPI(ci.currOwnerAPIClient)
			}			
			if err = userAPI.UpdateUserKey(deviceContext.GetOwner()); err != nil {
				return
			}
		}

		if ci.resetConfig {
			// register device with MyCS account service 
			// with new logged in user as owner. 
			if ci.currOwnerAPIClient != nil {
				// unregister this device using the prev owner's API client
				deviceAPI = mycscloud.NewDeviceAPI(ci.currOwnerAPIClient)
				if _, err = deviceAPI.UnRegisterDevice(ci.currDeviceID); err != nil {
					logger.DebugMessage(
						"initialize(): Unable to unregister device with ID '%s'. Registration of new device will continue: %s", 
						ci.currDeviceID, err.Error(),
					)
				}
			}
			// register new device with MyCS account service
			if ci.newOwnerAPIClient == nil {
				err = fmt.Errorf("no new owner API client available to register new device with")
				return
			}
			deviceAPI = mycscloud.NewDeviceAPI(ci.newOwnerAPIClient)
			if deviceIDKey, deviceID, err = deviceAPI.RegisterDevice(
				deviceName, 
				system.GetDeviceType(),
				system.GetDeviceVersion(clientType, clientVersion),
				"", 
				deviceContext.GetDevice().RSAPublicKey, 
				"",
			); err != nil {				
				return
			}
			deviceContext.SetDeviceID(deviceIDKey, deviceID, deviceName)
		}

		ci.appConfig.SetPassphrase(deviceLockPassphrase)
		ci.appConfig.SetKeyTimeout(time.Duration(unlockedTimeout) * time.Minute)
		ci.appConfig.SetInitialized()
		
		err = ci.appConfig.Save()
	}()
}
package auth

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"golang.org/x/oauth2"

	"github.com/appbricks/cloud-builder/auth"
	"github.com/appbricks/cloud-builder/config"
	"github.com/appbricks/cloud-builder/userspace"
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/ui"
	"github.com/mevansam/goutils/crypto"
	"github.com/mevansam/goutils/logger"
)

var callbackPorts = []int{9080, 19080, 29080, 39080, 49080, 59080}

type AuthRet struct{
	Error error
}

type TokenRet struct {
	AWSAuth *AWSCognitoJWT
	Error   error
}

type AsyncAuthRet chan AuthRet
type AsyncTokenRet chan TokenRet

func Authenticate(
	ctx context.Context,
	serviceConfig api.ServiceConfig, 
	authContext config.AuthContext, 
	appUI ui.UI, 
	loginMessages ...string,
) AsyncAuthRet {

	var (
		err error

		isAuthenticated bool
		authUrl string
	)

	authRet := make(AsyncAuthRet, 1)

	authn, cancelFunc := auth.NewAuthenticator(
		ctx,
		authContext,
		&oauth2.Config{
			ClientID:     serviceConfig.CliendID,
			ClientSecret: serviceConfig.ClientSecret,
			Scopes:       []string{"openid", "profile"},
			
			Endpoint: oauth2.Endpoint{
				AuthURL:  serviceConfig.AuthURL,
				TokenURL: serviceConfig.TokenURL,
			},
		}, 
		callBackHandler(),
	)

	if isAuthenticated, err = authn.IsAuthenticated(); err != nil && err.Error() != "not authenticated" {
		authRet <-AuthRet{err}
		return authRet
	}
	if !isAuthenticated {
		uh := appUI.NewUIMessageWithCancel("Login to My Cloud Space", cancelFunc)

		if len(loginMessages) > 0 {
			uh.WriteNoticeMessage(loginMessages[0])
		}
		if authUrl, err = authn.StartOAuthFlow(callbackPorts, logoRequestHandler); err != nil {
			logger.ErrorMessage("Authentication failed: %s", err.Error())	
			authRet <-AuthRet{err}
			return authRet
		}
		if err = openBrowser(authUrl); err != nil {
			logger.DebugMessage("ERROR! Unable to open browser for authentication: %s", err.Error())

			uh.WriteNoteMessage(
				"You need to open a browser window and navigate to the following URL in order to " +
				"login to your My Cloud Space account. Once authenticated the MyCS app will be ready " +
				"for use.",
			)
			uh.WriteText(fmt.Sprintf("\n=> %s", authUrl))

		} else {
			uh.WriteNoteMessage(
				"You have been directed to a browser window from which you need to login to your " +
				"My Cloud Space account. Once authenticated the MyCS app will be ready for use.",
			)
		}
		
		p := uh.ShowMessageWithProgressIndicator(
			"Waiting for authentication to complete.", "",
			"Authentication is complete. You are now signed in.", 0,
		)

		go func() {
			p.Start()
			defer p.Done()

			for wait := true; wait; {
				wait, err = authn.WaitForOAuthFlowCompletion(time.Second)
			}
			if err != nil {
				authRet <-AuthRet{err}
				return
			}
			// update app config with cloud properties
			cloudAPI := mycscloud.NewCloudAPI(api.NewGraphQLClient(serviceConfig.ApiURL, "", authContext))
			authRet <-AuthRet{cloudAPI.UpdateProperties(authContext)}
		}()

	} else {
		authRet <-AuthRet{nil}
	}
	return authRet
}

func callBackHandler() func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(authSuccessHTML)); err != nil {
			logger.DebugMessage("ERROR! Unable to return auth success open page.")
		}
	}
}

func logoRequestHandler() (string, func(http.ResponseWriter, *http.Request)) {
	return "/logo.png",
		func(w http.ResponseWriter, r *http.Request) {

			var (
				err  error
				data []byte
			)

			if data, err = base64.StdEncoding.DecodeString(appBricksLogoImg); err != nil {
				logger.DebugMessage("ERROR! Decoding logo image data.")
				return
			}
			if _, err = w.Write([]byte(data)); err != nil {
				logger.DebugMessage("ERROR! Unable to return logo image.")
			}
		}
}

func openBrowser(url string) (err error) {
	switch runtime.GOOS {
		case "linux":
			err = exec.Command("xdg-open", url).Run()
		case "windows":
			err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Run()
		case "darwin":
			err = exec.Command("open", url).Run()
		default:
			err = fmt.Errorf("unsupported platform")
	}
	return
}

func GetAuthenticatedToken(
	ctx context.Context,
	serviceConfig api.ServiceConfig,
	appConfig config.Config, 
	forceLogin bool, 
	appUI ui.UI, 
	loginMessages ...string,
) AsyncTokenRet {

	var (
		err error

		awsAuth *AWSCognitoJWT
	)
	tokenRet := make(AsyncTokenRet, 1)
	authContext := appConfig.AuthContext()
	if forceLogin {
		if err = authContext.Reset(); err != nil {
			tokenRet <-TokenRet{nil, err}
			return tokenRet
		}
	}

	authRet := Authenticate(ctx, serviceConfig, authContext, appUI, loginMessages...)
	go func() {
		defer close(authRet)
		if err = (<-authRet).Error; err != nil {
			logger.DebugMessage("ERROR! Authentication failed: %s", err.Error())	
			tokenRet <-TokenRet{nil, err}
			return
		}
		if awsAuth, err = NewAWSCognitoJWT(serviceConfig, authContext); err != nil {
			logger.DebugMessage("ERROR! Failed to extract auth token: %s", err.Error())	
			tokenRet <-TokenRet{nil, err}
			return
		}
		if err = appConfig.SetLoggedInUser(awsAuth.UserID(), awsAuth.Username()); err != nil {
			tokenRet <-TokenRet{nil, err}
			return
		}
		logger.TraceMessage("JWT Token for logged in user is: %# v", awsAuth.jwtToken)
		tokenRet <-TokenRet{awsAuth, err}
	}()

	return tokenRet
}

func ValidateAuthenticatedToken(
	serviceConfig api.ServiceConfig,
	appConfig config.Config, 
) (bool, error) {

	var (
		err error

		isAuthenticated bool
		awsAuth         *AWSCognitoJWT
	)

	authContext := appConfig.AuthContext()
	deviceContext := appConfig.DeviceContext()

	authn, _ := auth.NewAuthenticator(
		context.Background(),
		authContext,
		&oauth2.Config{
			ClientID:     serviceConfig.CliendID,
			ClientSecret: serviceConfig.ClientSecret,
			Scopes:       []string{"openid", "profile"},
			
			Endpoint: oauth2.Endpoint{
				AuthURL:  serviceConfig.AuthURL,
				TokenURL: serviceConfig.TokenURL,
			},
		}, 
		callBackHandler(),
	)
	if isAuthenticated, err = authn.IsAuthenticated(); err == nil {
		if awsAuth, err = NewAWSCognitoJWT(serviceConfig, authContext); 
			err == nil && awsAuth.Username() != deviceContext.GetLoggedInUserName() {
			
			if err = appConfig.SetLoggedInUser(awsAuth.UserID(), awsAuth.Username()); err != nil {
				logger.ErrorMessage("Failed to set logged in user: %s", err.Error())	
				isAuthenticated = false
			}

		} else {
			logger.ErrorMessage("Failed to extract auth token: %s", err.Error())	
			isAuthenticated = false
		}
	}
	return isAuthenticated, err
}

func Login(
	serviceConfig api.ServiceConfig,
	appConfig config.Config,
	appUI ui.UI, 
	handleLoginResult func(err error),
) {

	tokenRet := GetAuthenticatedToken(context.Background(), serviceConfig,appConfig, false, appUI)
	
	go func() {

		var (
			err error
		)

		defer close(tokenRet)
		token := <-tokenRet

		// always call handler on exit
		defer func() {
			if err != nil {
				if resetErr := appConfig.AuthContext().Reset(); resetErr != nil {
					logger.ErrorMessage(
						"Failed to reset auth context as device authorization failed: %s", 
						resetErr.Error(),
					)
				}	
			}			
			handleLoginResult(err)
		}()

		if err = token.Error; err == nil {
			err = AuthorizeDeviceAndUser(serviceConfig, appConfig, appUI);
		}		
	}()
}

func Logout(
	serviceConfig api.ServiceConfig,
	appConfig config.Config,
) error {

	var (
		err error

		awsAuth *AWSCognitoJWT
	)

	authContext := appConfig.AuthContext()
	if authContext.IsLoggedIn() {
		if awsAuth, err = NewAWSCognitoJWT(serviceConfig, authContext); err != nil {
			return err
		}
	}
	if err = authContext.Reset(); err != nil {
		return err
	}
	appConfig.DeviceContext().SetLoggedInUser("", "")
	
	if awsAuth != nil {
		logger.DebugMessage("User \"%s\" has been logged out.", awsAuth.Username())
	} else {
		logger.DebugMessage("Logout complete.")
	}
	return nil
}

func AuthorizeDeviceAndUser(
	serviceConfig api.ServiceConfig,
	appConfig config.Config,
	appUI ui.UI, 
) error {

	var (
		err, authErr error

		awsAuth *AWSCognitoJWT

		user *userspace.User

		userID, 
		userName,
		ownerUserID string
		ownerConfig []byte

		isOwnerSet bool

		keyFileName, keyFilePassphrase *string

		ownerKey *crypto.RSAKey
	)

	defer func() {
		if authErr != nil {
			appUI.ShowErrorMessage(fmt.Sprintf("Device authorization failed: %s", authErr.Error()))
		}
	}()

	authContext := appConfig.AuthContext()
	gqlClient := api.NewGraphQLClient(serviceConfig.ApiURL, "", authContext)

	deviceAPI := mycscloud.NewDeviceAPI(gqlClient)
	deviceContext := appConfig.DeviceContext()

	// ensure that the device has an owner
	if ownerUserID, isOwnerSet = deviceContext.GetOwnerUserID(); !isOwnerSet {
		err = fmt.Errorf("no device owner configured")
		return err
	}

	// validate and parse JWT token
	if awsAuth, err = NewAWSCognitoJWT(serviceConfig, authContext); err != nil {
		return err
	}
	userID = awsAuth.UserID()
	userName = awsAuth.Username()

	// authenticate device and user
	if authErr = deviceAPI.UpdateDeviceContext(deviceContext); authErr != nil {
		err = authErr
		
		authErrStr := authErr.Error()
		if authErrStr == "unauthorized(pending)" {
			appUI.ShowNoticeMessage(
				"Device Access",
				fmt.Sprintf("User \"%s\" is not authorized to use this device. A request to grant access to this device is still pending.", userName),
			)
			authErr = nil

		} else if authErrStr == "unauthorized" {
			requestAccess := make(chan bool)
			defer close(requestAccess)

			uh := appUI.NewUIMessage("Device Access")
			uh.WriteNoticeMessage(
				fmt.Sprintf(
					"User \"%s\" is not authorized to use this device.\n\n" +
					"Do you wish to request access to this device", userName,
				),
			)
			uh.ShowMessageWithYesNoInput(func(yes bool) {
				requestAccess <-yes
			})
			if <-requestAccess {
				if user, _ = deviceContext.GetGuestUser(userName); user == nil {
					if user, err = deviceContext.NewGuestUser(userID, userName); err != nil {
						err = fmt.Errorf("failed to add new guest user to device: %s", err.Error())
						return err
					}
				} else {
					user.Active = false
				}
				if _, _, err = deviceAPI.AddDeviceUser(deviceContext.GetDevice().DeviceID, ""); err != nil {
					err = fmt.Errorf("failed to add new guest user to device: %s", err.Error())
					return err
				}
				appUI.ShowNoteMessage(
					"Device Access",
					fmt.Sprintf("A request to grant user \"%s\" access to this device has been submitted.", user.Name),
				)
				authErr = nil

			} else {
				err = fmt.Errorf("access request declined")
			}
		}

		if resetErr := authContext.Reset(); resetErr != nil {
			logger.ErrorMessage(
				"Failed to reset auth context as device authorization failed: %s", 
				resetErr.Error(),
			)
		}
		return err
	}

	// if logged in user is the owner ensure 
	// owner is initialized and config is latest
	if userID == ownerUserID {
		owner := deviceContext.GetOwner()

		if len(owner.RSAPrivateKey) == 0 {
			input := make(chan *string, 1)
			defer close(input)
			
			uh := appUI.NewUIMessage("Open Device Owner's Key")			
			uh.ShowMessageWithFileInput(func(keyFileName *string) {
				input <-keyFileName
			})
			if keyFileName = <-input; keyFileName == nil {
				err = fmt.Errorf("no key file provided")
				return err
			}

			uh = appUI.NewUIMessage("Key File Passphrase")
			uh.WriteInfoMessage("Enter the passphrase needed to open the key file.")
			uh.ShowMessageWithSecureInput(func(keyFilePassphrase *string) {
				input <-keyFilePassphrase
			})
			if keyFilePassphrase = <-input; keyFilePassphrase == nil {
				err = fmt.Errorf("key file needs a passphrase")
				return err
			}

			if ownerKey, err = crypto.NewRSAKeyFromFile(*keyFileName, []byte(*keyFilePassphrase)); err == nil {
				err = owner.SetKey(ownerKey, false)				
			}
			if err != nil {
				err = fmt.Errorf("failed to load user's private key: %s", err.Error())
				return err
			}
		}
		
		if targetContext := appConfig.TargetContext(); targetContext != nil {
			if appConfig.GetConfigAsOf() < awsAuth.ConfigTimestamp() {
				userAPI := mycscloud.NewUserAPI(gqlClient)
		
				if ownerConfig, err = userAPI.GetUserConfig(owner); err != nil {
					err = fmt.Errorf(
						"failed to sync target context with remote: %s", 
						err.Error(),
					)
					return err
				}
				if err = appConfig.TargetContext().Reset(); err != nil {
					err = fmt.Errorf(
						"failed to sync target context with remote: %s", 
						err.Error(),
					)
					return err
				}
				if err = appConfig.TargetContext().Load(bytes.NewReader(ownerConfig)); err != nil {
					err = fmt.Errorf(
						"failed to sync target context with remote: %s", 
						err.Error(),
					)
					return err
				}
				appConfig.SetConfigAsOf(awsAuth.ConfigTimestamp())
			}
		}
	} 

	return nil
}

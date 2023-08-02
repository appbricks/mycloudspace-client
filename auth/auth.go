package auth

import (
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
	"github.com/appbricks/mycloudspace-client/api"
	"github.com/appbricks/mycloudspace-client/mycscloud"
	"github.com/appbricks/mycloudspace-client/ui"
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
	ui ui.UI, 
	loginMessages ...string,
) AsyncAuthRet {

	var (
		err error

		isAuthenticated bool
		authUrl string
	)

	authRet := make(AsyncAuthRet)

	authn := auth.NewAuthenticator(
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
		uh := ui.NewUIMessage("Login to My Cloud Space")

		if len(loginMessages) > 0 {
			fmt.Println()
			uh.WriteNoticeMessage(loginMessages[0])
		}
		if authUrl, err = authn.StartOAuthFlow(callbackPorts, logoRequestHandler); err != nil {
			logger.DebugMessage("ERROR! Authentication failed: %s", err.Error())	
			authRet <-AuthRet{err}
			return authRet
		}
		if err = openBrowser(authUrl); err != nil {
			logger.DebugMessage("ERROR! Unable to open browser for authentication: %s", err.Error())

			fmt.Println()
			uh.WriteNoteMessage(
				"You need to open a browser window and navigate to the following URL in order to " +
				"login to your My Cloud Space account. Once authenticated the MyCS app will be ready " +
				"for use.",
			)
			uh.WriteText(fmt.Sprintf("\n=> %s\n\n", authUrl))

		} else {
			fmt.Println()
			uh.WriteNoteMessage(
				"You have been directed to a browser window from which you need to login to your " +
				"My Cloud Space account. Once authenticated the MyCS app will be ready for use.",
			)
			fmt.Println()
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
	ui ui.UI, 
	loginMessages ...string,
) AsyncTokenRet {

	var (
		err error

		awsAuth *AWSCognitoJWT
	)
	tokenRet := make(AsyncTokenRet)
	authContext := appConfig.AuthContext()
	if forceLogin {
		if err = authContext.Reset(); err != nil {
			tokenRet <-TokenRet{nil, err}
			return tokenRet
		}
	}

	authRet := Authenticate(ctx, serviceConfig, authContext, ui, loginMessages...)
	go func() {
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
		tokenRet <-TokenRet{awsAuth, err}
	}()

	return tokenRet
}

// func AuthorizeDeviceAndUser(config config.Config) error {

// 	var (
// 		err error

// 		awsAuth *AWSCognitoJWT

// 		user *userspace.User

// 		userID, 
// 		userName,
// 		ownerUserID string
// 		ownerConfig []byte

// 		ownerKey *crypto.RSAKey

// 		requestAccess bool
// 	)

// 	deviceAPI := mycscloud.NewDeviceAPI(api.NewGraphQLClient(cbcli_config.AWS_USERSPACE_API_URL, "", config))
// 	deviceContext := config.DeviceContext()

// 	// validate and parse JWT token
// 	if awsAuth, err = NewAWSCognitoJWT(config); err != nil {
// 		return err
// 	}
// 	userID = awsAuth.UserID()
// 	userName = awsAuth.Username()

// 	// authenticate device and user
// 	if err = deviceAPI.UpdateDeviceContext(deviceContext); err != nil {

// 		errStr := err.Error()
// 		if errStr == "unauthorized(pending)" {
// 			fmt.Println()
// 			cbcli_utils.ShowNoticeMessage("User \"%s\" is not authorized to use this device. A request to grant access to this device is still pending.", userName)

// 		} else if errStr == "unauthorized" {
// 			fmt.Println()
// 			cbcli_utils.ShowNoticeMessage("User \"%s\" is not authorized to use this device.", userName)			
// 			fmt.Println()
			
// 			requestAccess = cbcli_utils.GetYesNoUserInput("Do you wish to request access to this device : ", false)			
// 			if (requestAccess) {
// 				if user, _ = deviceContext.GetGuestUser(userName); user == nil {
// 					if user, err = deviceContext.NewGuestUser(userID, userName); err != nil {
// 						return err
// 					}
// 				} else {
// 					user.Active = false
// 				}
// 				if _, _, err = deviceAPI.AddDeviceUser(deviceContext.GetDevice().DeviceID, ""); err != nil {
// 					return err
// 				}
// 				fmt.Println()
// 				cbcli_utils.ShowNoticeMessage("A request to grant user \"%s\" access to this device has been submitted.", user.Name)

// 			} else {
// 				return fmt.Errorf("access request declined")
// 			}
			
// 			return nil
// 		} else {
// 			return err
// 		}
// 	}

// 	// ensure that the device has an owner
// 	ownerUserID, isOwnerSet := deviceContext.GetOwnerUserID()
// 	if !isOwnerSet {
// 		fmt.Println()
// 		cbcli_utils.ShowCommentMessage(
// 			"This Cloud Builder CLI device has not been initialized. You can do this by running " +
// 			"the \"cb init\" command and claiming the device by logging in as the device owner.",
// 		)
// 		fmt.Println()
// 		os.Exit(1)
// 	}

// 	// if logged in user is the owner ensure 
// 	// owner is intialized and config is latest
// 	if userID == ownerUserID {
// 		owner := deviceContext.GetOwner()

// 		if len(owner.RSAPrivateKey) == 0 {
// 			fmt.Println()

// 			line := liner.NewLiner()
// 			line.SetCtrlCAborts(true)
// 			if ownerKey, err = ImportPrivateKey(line); err != nil {
// 				cbcli_utils.ShowErrorAndExit(
// 					fmt.Sprintf("User's private key import failed with error: %s", err.Error()),
// 				)
// 			}
// 			if err = owner.SetKey(ownerKey); err != nil {
// 				cbcli_utils.ShowErrorAndExit("Failed to validate provided private key with user's known public key.")
// 			}		
// 		}
// 		if config.GetConfigAsOf() < awsAuth.ConfigTimestamp() {
// 			userAPI := mycscloud.NewUserAPI(api.NewGraphQLClient(cbcli_config.AWS_USERSPACE_API_URL, "", config))
	
// 			if ownerConfig, err = userAPI.GetUserConfig(owner); err != nil {
// 				return err
// 			}
// 			if err = config.TargetContext().Reset(); err != nil {
// 				cbcli_utils.ShowErrorAndExit(
// 					fmt.Sprintf(
// 						"Failed to reset current config as a change was detected: %s", 
// 						err.Error(),
// 					),
// 				)
// 			}
// 			if err = config.TargetContext().Load(bytes.NewReader(ownerConfig)); err != nil {
// 				cbcli_utils.ShowErrorAndExit(
// 					fmt.Sprintf(
// 						"Failed to reset load updated config: %s", 
// 						err.Error(),
// 					),
// 				)
// 			}
// 			config.SetConfigAsOf(awsAuth.ConfigTimestamp())
// 		}
// 	} 

// 	return nil
// }

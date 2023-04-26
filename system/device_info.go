package system

import (
	"fmt"
	"runtime"
	"time"

	"github.com/mevansam/goutils/logger"
)

var (
	deviceType string
	deviceTypeC chan string
)

func GetDeviceType() string {
	if len(deviceType) == 0 {
		select {
		case deviceType = <-deviceTypeC:
		case <-time.After(time.Second):
			logger.ErrorMessage("system.GetDeviceType(): Timed out waiting to determine system device type.")
			deviceType = "Unknown"
		}		
	}
	return deviceType;
}

func GetDeviceVersion(clientType string, clientVersion string) string {
	return fmt.Sprintf("%s/%s:%s/%s", clientType, runtime.GOOS, runtime.GOARCH, clientVersion)
}

func init() {
	deviceTypeC = make(chan string)
}

//go:build darwin
// +build darwin

package system

import (
	"bytes"
	"fmt"
	"regexp"

	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/run"
	"github.com/mevansam/goutils/utils"
	"github.com/mitchellh/go-homedir"
)

var (
	productNamePattern = regexp.MustCompile(`^ProductName:\s+(.*)\s*$`)
	productVersionPattern = regexp.MustCompile(`^ProductVersion:\s+(.*)\s*$`)
	modelNamePattern = regexp.MustCompile(`^\s*Model Name:\s+(.*)\s*$`)
)

func init() {

	// asynchronously determine device type 
	// via available system utilities
	go func() {
		var (
			err error
	
			swVers,
			systemProfiler run.CLI
			outputBuffer   bytes.Buffer

			results map[string][][]string
	
			productName, 
			productVersion string			
		)		
		
		deviceType := "DarwinPC"
		home, _ := homedir.Dir()
	
		if systemProfiler, err = run.NewCLI("/usr/sbin/system_profiler", home, &outputBuffer, &outputBuffer); err != nil {
			logger.ErrorMessage("system.init:darwin(): Error creating CLI for /usr/bin/system_profiler: %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		if err = systemProfiler.Run([]string{"SPHardwareDataType"}); err != nil {
			logger.ErrorMessage("system.init:darwin(): Error running \"system_profiler SPHardwareDataType\": %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		results = utils.ExtractMatches(outputBuffer.Bytes(), map[string]*regexp.Regexp{
			"modelName": modelNamePattern,
		})
		if results["modelName"] != nil && len(results["modelName"][0]) == 2 {
			modelName := results["modelName"][0][1]
			deviceTypeC <- modelName
			return
		}

		outputBuffer.Reset()
		if swVers, err = run.NewCLI("/usr/bin/sw_vers", home, &outputBuffer, &outputBuffer); err != nil {
			logger.ErrorMessage("system.init:darwin(): Error creating CLI for /usr/bin/sw_vers: %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		if err = swVers.Run([]string{}); err != nil {
			logger.ErrorMessage("system.init:darwin(): Error running \"sw_vers\": %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		results = utils.ExtractMatches(outputBuffer.Bytes(), map[string]*regexp.Regexp{
			"productName": productNamePattern,
			"productVersion": productVersionPattern,
		})		
		if results["productVersion"] != nil && len(results["productVersion"][0]) > 1 {
			productVersion = results["productVersion"][0][1]
		}
		if results["productName"] == nil || len(results["productName"][0]) < 1 {
			logger.ErrorMessage("system.init:darwin(): Error unable to determine device product name")
			deviceTypeC <- deviceType
			return
		}
		productName = results["productName"][0][1]
		deviceTypeC <- fmt.Sprintf("DarwinPC/%s:%s", productName, productVersion)
	}()
}

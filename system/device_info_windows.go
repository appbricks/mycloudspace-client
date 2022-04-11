//go:build windows
// +build windows

package system

import (
	"bytes"
	"strings"
	"regexp"

	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/run"
	"github.com/mevansam/goutils/utils"
	"github.com/mitchellh/go-homedir"
)

var (
	systemModelPattern  = regexp.MustCompile(`^System Model:\s+(.*)\s*$`)
	hyperVReqmtsPattern = regexp.MustCompile(`^Hyper-V Requirements:\s+(.*)\s*$`)
)

func init() {

	// asynchronously determine device type 
	// via available system utilities
	go func() {
		var (
			err error
			
			systemInfo   run.CLI
			outputBuffer bytes.Buffer	

			results map[string][][]string

			systemModel, hyperVReqmts string
		)
		
		deviceType := "WindowsPC"
		home, _ := homedir.Dir()

		if systemInfo, err = run.NewCLI("C:/Windows/System32/systeminfo.exe", home, &outputBuffer, &outputBuffer); err != nil {
			logger.ErrorMessage("system.init:windows(): Error creating CLI for C:/Windows/System32/systeminfo.exe: %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		if err = systemInfo.Run([]string{}); err != nil {
			logger.ErrorMessage("system.init:windows(): Error running \"systeminfo.exe\": %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		results = utils.ExtractMatches(outputBuffer.Bytes(), map[string]*regexp.Regexp{
			"systemModel": systemModelPattern,
			"hyperVReqmts": hyperVReqmtsPattern,
		})
		if results["systemModel"] != nil && len(results["systemModel"][0]) == 2 {
			systemModel = results["systemModel"][0][1]
		}
		if results["hyperVReqmts"] != nil && len(results["hyperVReqmts"][0]) == 2 {
			hyperVReqmts = results["hyperVReqmts"][0][1]
		}

		if strings.HasPrefix(hyperVReqmts, "A hypervisor has been detected.") {
			if strings.HasPrefix(systemModel, "VirtualBox") {
				deviceTypeC <- "WindowsVM/VirtualBox"
			}	else if strings.HasPrefix(systemModel, "VMware") {
				deviceTypeC <- "WindowsVM/VMware"
			} else {
				deviceTypeC <- "WindowsVM"
			}	
		} else {
			deviceTypeC <- deviceType
		}
	}()
}

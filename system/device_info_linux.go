//go:build linux
// +build linux

package system

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mevansam/goutils/logger"
	"github.com/mevansam/goutils/run"
	"github.com/mitchellh/go-homedir"
)

func init() {

	// asynchronously determine device type 
	// via available system utilities
	go func() {
		var (
			err error
	
			systemDetectVirt run.CLI
			outputBuffer     bytes.Buffer	
		)		
		
		deviceType := "LinuxPC"
		home, _ := homedir.Dir()

		if systemDetectVirt, err = run.NewCLI("/usr/bin/systemd-detect-virt", home, &outputBuffer, &outputBuffer); err != nil {
			logger.ErrorMessage("system.init:linux(): Error creating CLI for /usr/bin/systemd-detect-virt: %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		if err = systemDetectVirt.Run([]string{}); err != nil {
			logger.ErrorMessage("system.init:linux(): Error running \"systemd-detect-virt\": %s", err.Error())
			deviceTypeC <- deviceType
			return
		}
		
		virtType := strings.TrimSpace(outputBuffer.String())
		switch virtType {
		case "none":
			deviceTypeC <- deviceType
		case "oracle":
			deviceTypeC <- "LinuxVM/virtualbox"
		case "openvz",
			"lxc", 
			"lxc-libvirt", 
			"systemd-nspawn",
			"docker", 
			"podman",
			"rkt",
			"wsl",
			"proot",
			"pouch":
			deviceTypeC <- fmt.Sprintf("LinuxContainer/%s", virtType)
		default:
			deviceTypeC <- fmt.Sprintf("LinuxVM/%s", virtType)
		}
	}()
}

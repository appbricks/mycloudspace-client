package network

import (
	"net"
	"regexp"
	"strconv"
)

// given a device name prefix return the next available one
func GetNextAvailabeInterface(prefix string) (string, error) {

	var (
		err error

		devNamePattern *regexp.Regexp
		matches        [][]string

		ifaces   []net.Interface
		devIndex, maxIndex int
	)

	if devNamePattern, err = regexp.Compile("^" + prefix + "([0-9]+)$"); err != nil {
		return "", err
	}
	if ifaces, err = net.Interfaces(); err != nil {
		return "", err
	}
	maxIndex = 0
	for _, i := range ifaces {

		if matches = devNamePattern.FindAllStringSubmatch(i.Name, -1); matches != nil {
			if devIndex, err = strconv.Atoi(matches[0][1]); err != nil {
				continue
			}
			if maxIndex < devIndex {
				maxIndex = devIndex
			}
		}
	}
	return prefix + strconv.FormatInt(int64(maxIndex+1), 10), nil
}

func IncIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}
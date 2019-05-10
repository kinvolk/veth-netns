// Copyright 2019 Kinvolk GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// getNSIno takes namespace type and a pid and returns the namespace inode
// number
func getNSIno(nsType, pid string) (int, error) {
	nsPath := filepath.Join("/proc", pid, "ns", nsType)
	i, err := os.Readlink(nsPath)
	if err != nil {
		return -1, err
	}

	// net:[4026532431] -> 4026531837
	inoStr := strings.Split(i, ":")[1]
	inoStr = inoStr[1 : len(inoStr)-1]

	ino, err := strconv.Atoi(inoStr)
	if err != nil {
		return -1, err
	}

	return ino, nil
}

// getAllNetNS goes through /proc/* and returns a map from network namespace
// inode number to its path for all the network namespaces in all processes
func getAllNetNS() (map[int]string, error) {
	netNamespaces := make(map[int]string)
	procDirs, _ := filepath.Glob("/proc/[0-9]*")

	for _, f := range procDirs { // each file representing a process
		_, pid := filepath.Split(f)

		ns, err := getNSIno("net", pid)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error resolving %v: %v\n", pid, err)
			continue
		}

		// inode number -> path
		netNamespaces[ns] = filepath.Join(f, "ns", "net")
	}

	return netNamespaces, nil
}

type ifInfo struct {
	Ifindex     int    `json:"ifindex"`
	IFName      string `json:"ifname"`
	LinkNetnsId int    `json:"link_netnsid"`
}

// getLinkNetNSID gets the link-netnsid of vethName by calling the ip command
func getLinkNetNSID(vethName string) (int, error) {
	prog := "ip"
	args := []string{
		"--json",
		"link",
		"show",
		vethName}

	output, err := exec.Command(prog, args...).CombinedOutput()
	if err != nil {
		return -1, fmt.Errorf("failed to get link-netnsid from veth %v: %v\n%v", vethName, err, output)
	}

	m := []ifInfo{}
	err = json.Unmarshal(output, &m)
	if err != nil {
		return -1, fmt.Errorf("cannot parse json output: %v\n%s", err, output)
	}
	return m[0].LinkNetnsId, nil
}

// getLocalNetNSID gets the local network namespace ID of netNSPath through
// netlink from the point of view of the current network namespace
func getLocalNetNSID(netNSPath string) (int, error) {
	f, err := os.Open(netNSPath)
	if err != nil {
		return -1, err
	}
	defer f.Close()

	fd := f.Fd()

	localNetNSID, err := netlink.GetNetNsIdByFd(int(fd))
	if err != nil {
		return -1, err
	}
	return localNetNSID, nil
}

// getNetNSFromVeth takes a veth name and returns the network namespace inode
// number of the other end of the veth pair
func getNetNSFromVeth(vethName string) (int, error) {
	linkNetNSID, err := getLinkNetNSID(vethName)
	if err != nil {
		return -1, err
	}
	fmt.Fprintf(os.Stdout, "link-netnsid of %s: %d\n", vethName, linkNetNSID)

	netNSs, err := getAllNetNS()
	if err != nil {
		return -1, err
	}

	origNS, err := netns.Get()
	if err != nil {
		return -1, err
	}
	defer origNS.Close()

	for _, netNSPath := range netNSs {
		nsHandle, err := netns.GetFromPath(netNSPath)
		if err != nil {
			return -1, err
		}
		defer nsHandle.Close()

		if err := netns.Set(nsHandle); err != nil {
			return -1, err
		}
		defer netns.Set(origNS)

		for netNSIno, netNSPath2 := range netNSs {
			localNetNSID, err := getLocalNetNSID(netNSPath2)
			if err != nil {
				return -1, err
			}

			if localNetNSID == linkNetNSID {
				return netNSIno, nil
			}
		}
	}

	return -1, fmt.Errorf("netns inode for %q not found", vethName)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s VETH_NAME\n", os.Args[0])
		os.Exit(1)
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	netNS, err := getNetNSFromVeth(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "netns ino ID for %v: %v\n", os.Args[1], netNS)
}

package main

import (
	"gopkg.in/urfave/cli.v1"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

var (
	dataDirFlag = cli.StringFlag{
		Name:  "data-dir",
		Value: defaultDataDir(),
		Usage: "directory for block-chain databases",
	}

	networkFlag = cli.StringFlag{
		Name:  "network",
		Usage: "the network to join (main|test|warringstakes)",
	}
)

// copy from go-ethereum
func defaultDataDir() string {
	// Try to place the data folder in the user's home dir
	if home := homeDir(); home != "" {
		if runtime.GOOS == "darwin" {
			return filepath.Join(home, "Library", "Application Support", "org.dfinlab.meter")
		} else if runtime.GOOS == "windows" {
			return filepath.Join(home, "AppData", "Roaming", "org.dfinlab.meter")
		} else {
			return filepath.Join(home, ".org.dfinlab.meter")
		}
	}
	// As we cannot guess a stable location, return empty and handle later
	return ""
}

func homeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

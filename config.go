// Copyright (c) 2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaspanet/dnsseeder/version"
	"github.com/kaspanet/kaspad/config"
	"github.com/pkg/errors"

	"github.com/jessevdk/go-flags"
	"github.com/kaspanet/kaspad/util"
)

const (
	defaultConfigFilename = "dnsseeder.conf"
	defaultLogFilename    = "dnsseeder.log"
	defaultErrLogFilename = "dnsseeder_err.log"
	defaultListenPort     = "5354"
)

var (
	// Default configuration options
	defaultHomeDir    = util.AppDataDir("dnsseeder", false)
	defaultConfigFile = filepath.Join(defaultHomeDir, defaultConfigFilename)
	defaultLogFile    = filepath.Join(defaultHomeDir, defaultLogFilename)
	defaultErrLogFile = filepath.Join(defaultHomeDir, defaultErrLogFilename)
)

var activeConfig *ConfigFlags

// ActiveConfig returns the active configuration struct
func ActiveConfig() *ConfigFlags {
	return activeConfig
}

// ConfigFlags holds the configurations set by the command line argument
type ConfigFlags struct {
	ShowVersion bool   `short:"V" long:"version" description:"Display version information and exit"`
	Host        string `short:"H" long:"host" description:"Seed DNS address"`
	Listen      string `long:"listen" short:"l" description:"Listen on address:port"`
	Nameserver  string `short:"n" long:"nameserver" description:"hostname of nameserver"`
	Seeder      string `short:"s" long:"default-seeder" description:"IP address of a  working node"`
	Profile     string `long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	config.NetworkFlags
}

func loadConfig() (*ConfigFlags, error) {
	err := os.MkdirAll(defaultHomeDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is
		// linked to a directory that does not exist (probably because
		// it's not mounted).
		var pathErr *os.PathError
		if ok := errors.As(err, &pathErr); ok && os.IsExist(err) {
			if link, linkErr := os.Readlink(pathErr.Path); linkErr == nil {
				str := "is symlink %s -> %s mounted?"
				err = errors.Errorf(str, pathErr.Path, link)
			}
		}

		str := "failed to create home directory: %v"
		err := errors.Wrap(err, str)
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	// Default config.
	activeConfig = &ConfigFlags{
		Listen: normalizeAddress("localhost", defaultListenPort),
	}

	preCfg := activeConfig
	preParser := flags.NewParser(preCfg, flags.Default)
	_, err = preParser.Parse()
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type == flags.ErrHelp {
			os.Exit(0)
		}
		preParser.WriteHelp(os.Stderr)
		return nil, err
	}

	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))

	// Show the version and exit if the version flag was specified.
	if preCfg.ShowVersion {
		fmt.Println(appName, "version", version.Version())
		os.Exit(0)
	}

	// Load additional config from file.
	parser := flags.NewParser(activeConfig, flags.Default)
	err = flags.NewIniParser(parser).ParseFile(defaultConfigFile)
	if err != nil {
		var pathErr *os.PathError
		if !errors.As(err, &pathErr) {
			fmt.Fprintf(os.Stderr, "Error parsing ConfigFlags "+
				"file: %v\n", err)
			fmt.Fprintf(os.Stderr, "Use `%s -h` to show usage\n", appName)
			return nil, err
		}
	}

	// Parse command line options again to ensure they take precedence.
	_, err = parser.Parse()
	if err != nil {
		var flagsErr *flags.Error
		if errors.As(err, &flagsErr) && flagsErr.Type != flags.ErrHelp {
			parser.WriteHelp(os.Stderr)
		}
		return nil, err
	}

	if len(activeConfig.Host) == 0 {
		str := "Please specify a hostname"
		err := errors.Errorf(str)
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	if len(activeConfig.Nameserver) == 0 {
		str := "Please specify a nameserver"
		err := errors.Errorf(str)
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	activeConfig.Listen = normalizeAddress(activeConfig.Listen, defaultListenPort)

	err = activeConfig.ResolveNetwork(parser)
	if err != nil {
		return nil, err
	}

	if activeConfig.Profile != "" {
		profilePort, err := strconv.Atoi(activeConfig.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			return nil, errors.New("The profile port must be between 1024 and 65535")
		}
	}

	initLog(defaultLogFile, defaultErrLogFile)

	return activeConfig, nil
}

// normalizeAddress returns addr with the passed default port appended if
// there is not already a port specified.
func normalizeAddress(addr, defaultPort string) string {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		return net.JoinHostPort(addr, defaultPort)
	}
	return addr
}

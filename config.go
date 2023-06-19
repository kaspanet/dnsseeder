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

	"github.com/kaspanet/kaspad/infrastructure/config"

	"github.com/kaspanet/dnsseeder/version"
	"github.com/pkg/errors"

	"github.com/jessevdk/go-flags"
	"github.com/kaspanet/kaspad/util"
)

const (
	defaultConfigFilename = "dnsseeder.conf"
	defaultLogFilename    = "dnsseeder.log"
	defaultErrLogFilename = "dnsseeder_err.log"
	defaultListenPort     = "5354"
	defaultGrpcListenPort = "3737"
)

var (
	// DefaultAppDir is the default home directory for dnsseeder.
	DefaultAppDir = util.AppDir("dnsseeder", false)

	defaultConfigFile = filepath.Join(DefaultAppDir, defaultConfigFilename)
)

var activeConfig *ConfigFlags

// ActiveConfig returns the active configuration struct
func ActiveConfig() *ConfigFlags {
	return activeConfig
}

// ConfigFlags holds the configurations set by the command line argument
type ConfigFlags struct {
	AppDir      string `short:"b" long:"appdir" description:"Directory to store data"`
	KnownPeers  string `short:"p" long:"peers" description:"List of already known peer addresses"`
	ShowVersion bool   `short:"V" long:"version" description:"Display version information and exit"`
	Host        string `short:"H" long:"host" description:"Seed DNS address"`
	Listen      string `long:"listen" short:"l" description:"Listen on address:port"`
	Nameserver  string `short:"n" long:"nameserver" description:"hostname of nameserver"`
	Seeder      string `short:"s" long:"default-seeder" description:"IP address of a working node, optionally with a port specifier"`
	Profile     string `long:"profile" description:"Enable HTTP profiling on given port -- NOTE port must be between 1024 and 65536"`
	GRPCListen  string `long:"grpclisten" description:"Listen gRPC requests on address:port"`
	NetSuffix   uint16 `long:"netsuffix" description:"Testnet network suffix number"`
	config.NetworkFlags
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(DefaultAppDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but they variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// Try to build the provided path if it does not exist yet.
func createPathIfNeeded(path string) error {
	err := os.MkdirAll(path, 0700)
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
		return err
	}
	return nil
}

func loadConfig() (*ConfigFlags, error) {
	// Default config.
	activeConfig = &ConfigFlags{
		AppDir:     DefaultAppDir,
		Listen:     normalizeAddress("localhost", defaultListenPort),
		GRPCListen: normalizeAddress("localhost", defaultGrpcListenPort),
		NetSuffix:  10,
	}

	preCfg := activeConfig
	preParser := flags.NewParser(preCfg, flags.Default)
	_, err := preParser.Parse()
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

	// Manually enforce testnet 11 net params so we do not have to 
	// support this special network in kaspad.
	if activeConfig.NetSuffix != 0 {
		if !activeConfig.Testnet {
			return nil, errors.New("The net suffix can only be used with testnet")
		}
		if activeConfig.NetSuffix != 11 {
			return nil, errors.New("The only supported explicit testnet net suffix is 11")
		}
		activeConfig.NetParams().DefaultPort = "16311";
		activeConfig.NetParams().Name = "kaspa-testnet-11";
	}

	activeConfig.AppDir = cleanAndExpandPath(activeConfig.AppDir)
	// Append the network type to the app directory so it is "namespaced"
	// per network.
	// All data is specific to a network, so namespacing the data directory
	// means each individual piece of serialized data does not have to
	// worry about changing names per network and such.
	activeConfig.AppDir = filepath.Join(activeConfig.AppDir, activeConfig.NetParams().Name)

	appLogFile := filepath.Join(activeConfig.AppDir, defaultLogFilename)
	appErrLogFile := filepath.Join(activeConfig.AppDir, defaultErrLogFilename)

	err = createPathIfNeeded(activeConfig.AppDir)
	if err != nil {
		return nil, err
	}

	if activeConfig.Profile != "" {
		profilePort, err := strconv.Atoi(activeConfig.Profile)
		if err != nil || profilePort < 1024 || profilePort > 65535 {
			return nil, errors.New("The profile port must be between 1024 and 65535")
		}
	}

	initLog(appLogFile, appErrLogFile)

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

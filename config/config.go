package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/prashantv/autobld/log"
	"github.com/prashantv/autobld/proxy"

	goflags "github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v2"
)

var defaultExcludeDirMap = map[string]bool{".git": true, ".hg": true}

// ConfigFile is the struct defining the config file passed in to the file watcher.
type Config struct {
	// BaseDir is the base directory where configs are based.
	// If this is not specified, the config file's location is used by default.
	BaseDir string `yaml:"baseDir"`

	// Matchers is the list of configurations to match.
	// If none are specified, it defaults to looking in baseDir for any changes.
	Matchers []Matcher `yaml:"matchers"`

	// ProxyConfigs is the list of ports that the file watcher listens on and forwards.
	ProxyConfigs []proxy.Config `yaml:"proxy"`

	// Action is the command to run to compile + restart the server.
	Action []string `yaml:"action"`

	configsMap map[string]*Matcher
}

// Matcher represents a specific set of patterns for some directories.
type Matcher struct {
	Patterns []string `yaml:"patterns"`
	Dirs     []string `yaml:"dirs"`

	// ExcludeDir is the name of directories that are excluded from the watcher.
	// By default, everything in defaultExcludeDirMap is excluded.
	ExcludeDir []string `yaml:"excludeDir"`

	excludeDirMap map[string]bool
}

// opts are the command-line flags parsed by go-flags.
type opts struct {
	Verbose []bool `long:"verbose" short:"v" description:"Verbose logging"`
	Quiet   bool   `long:"quiet" short:"q" description:"Turns off all logging"`

	// If ConfigPath is set, then all arguments under == Config == are ignored.
	ConfigPath string `long:"config" short:"c" description:"Config file path"`
	// == Config ==
	ExcludeDirs []string `long:"excludeDir" short:"x" description:"Directory names to exclude" default:".git,.hg"`
	BaseDir     string   `long:"dir" short:"d" description:"Directory to run commands in"`
	Proxies     []string `long:"proxy" short:"p" description:"Proxy ports, specified as [protocol]:[sourcePort]:[targetPort]/[targetBaseDir]"`
	Args        struct {
		Action []string `positional-arg-name:"Action and arguments" description:"Action and arguments to run"`
	} `positional-args:"yes" required:"yes"`
}

func Parse() (*Config, error) {
	opts := &opts{}
	if _, err := goflags.Parse(opts); err != nil {
		return nil, err
	}
	if opts.Quiet {
		log.SetLevel(-1)
	} else {
		log.SetLevel(len(opts.Verbose))
	}
	if opts.ConfigPath == "" {
		return parseArgs(opts)
	}
	return parseFile(opts.ConfigPath)
}

func normalize(config *Config) (*Config, error) {
	if len(config.Action) == 0 {
		return nil, errors.New("no action specified, please specify an action")
	}

	// Set up a default dir config to listen for everything.
	if len(config.Matchers) == 0 {
		config.Matchers = []Matcher{{
			Patterns: []string{"*"},
		}}
	}
	config.configsMap = make(map[string]*Matcher)

	for i := range config.Matchers {
		if len(config.Matchers[i].ExcludeDir) == 0 {
			config.Matchers[i].excludeDirMap = defaultExcludeDirMap
		} else {
			m := make(map[string]bool)
			for _, dir := range config.Matchers[i].ExcludeDir {
				m[dir] = true
			}
			config.Matchers[i].excludeDirMap = m
		}
	}
	log.V("Initializing with config: %+v", config)
	return config, nil
}

func parseArgs(opts *opts) (*Config, error) {
	c := &Config{}
	c.Action = opts.Args.Action
	c.BaseDir = opts.BaseDir

	if c.BaseDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		c.BaseDir = wd
	}
	for _, p := range opts.Proxies {
		pConfig, err := proxy.Parse(p)
		if err != nil {
			return nil, err
		}
		c.ProxyConfigs = append(c.ProxyConfigs, pConfig)
	}
	return normalize(c)
}

func parseFile(configPath string) (*Config, error) {
	config := &Config{}
	bytes, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %v", err)
	}
	if err := yaml.Unmarshal(bytes, config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %v", err)
	}
	// Relative BaseDir is relative to the config file location.
	if !filepath.IsAbs(config.BaseDir) {
		config.BaseDir = filepath.Join(filepath.Dir(configPath), config.BaseDir)
	}
	return normalize(config)
}
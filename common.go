package main

import (
	"io/ioutil"
	"log"
	"regexp"
	"sort"

	"github.com/davecgh/go-spew/spew"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
)

const (
	configPath    = "/defaults/config.yaml"
	logLevelDebug = "debug"
)

// this tool runs in three modes:
// controller: which runs in source cluster and keeps looking for new services to sync with peers
// target: which runs in destination cluster, listens for requests from controller and creates/updates services as received
// sync: which runs in both controller as well as target
const (
	runModeController = "controller"
	runModeTarget     = "target"
	runModeSync       = "sync"
)

type config struct {
	RunMode           string           `yaml:"runMode"`
	LogLevel          string           `yaml:"logLevel"`
	Peers             []string         `yaml:"peers"`
	SyncNamespaces    sort.StringSlice `yaml:"syncNamespaces"`
	SkipServicesRegex string           `yaml:"skipServiceRegex"`
}

var (
	defaultConfig *config
)

func readConfig() {
	configData, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Fatalf("failed to read config file %s - %v", configPath, err)
	}
	defaultConfig = new(config)
	err = yaml.UnmarshalStrict(configData, defaultConfig)
	if err != nil {
		log.Fatalf("failed to process config %s - %v", configPath, err)
	}

	switch defaultConfig.RunMode {
	case runModeController:
	case runModeSync:
	case runModeTarget:
	default:
		log.Fatalf("runMode must be either '%s', '%s' or '%s', found '%s'.", runModeController, runModeTarget, runModeSync, defaultConfig.RunMode)
	}

	regexp.MustCompile(defaultConfig.SkipServicesRegex)
	sort.Sort(defaultConfig.SyncNamespaces)
	debugSpew(defaultConfig)
}

func loadConfig() {
	readConfig()

	// watch for changes in config
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				debug("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					readConfig()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				debug("config watcher error:", err)
			}
		}
	}()
	err = watcher.Add(configPath)
	if err != nil {
		log.Fatalf("failed to add watcher for config changes: %v", err)
	}
}

func debug(msg string, args ...interface{}) {
	if defaultConfig.LogLevel == logLevelDebug {
		log.Printf("<DEBUG> "+msg+" \n", args...)
	}
}

func debugSpew(a ...interface{}) {
	if defaultConfig.LogLevel == logLevelDebug {
		log.Printf("<DEBUG> <dump>\n%s\n</dump>\n", spew.Sdump(a...))
	}
}

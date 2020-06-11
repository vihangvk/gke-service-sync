package main

import (
	"io/ioutil"
	"log"
	"path/filepath"
	"regexp"
	"sort"
	"sync"

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
	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			log.Fatalf("failed to create new watcher for config changes: %v", err)
		}
		defer watcher.Close()
		filePath, _ := filepath.EvalSymlinks(configPath)
		wg := sync.WaitGroup{}
		go func() {
			defer wg.Done()
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						debug("config change event closed")
						return
					}
					debug("event op: '%s', name: '%s", event.Op.String(), event.Name)
					if event.Op == fsnotify.Remove {
						watcher.Remove(event.Name)
						filePath, _ = filepath.EvalSymlinks(configPath)
						watcher.Add(filePath)
						readConfig()
					}
					if event.Op&fsnotify.Write == fsnotify.Write {
						readConfig()
					}
				case err, ok := <-watcher.Errors:
					if ok {
						log.Printf("config watcher error: %v", err)
						return
					}
				}
			}
		}()
		err = watcher.Add(filePath)
		if err != nil {
			log.Fatalf("failed to add watcher for config changes: %v", err)
		}
		wg.Add(1)
		wg.Wait()
	}()
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

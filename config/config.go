package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/toml"
	"github.com/influxdata/toml/ast"
	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/internal"
	"github.com/shanas-swi/telegraf-v1.16.3/models"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/aggregators"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/outputs"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/parsers"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/processors"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/serializers"
)

var (
	// Default sections
	sectionDefaults = []string{"global_tags", "agent", "outputs",
		"processors", "aggregators", "inputs"}

	// Default input plugins
	inputDefaults = []string{"cpu", "mem", "swap", "system", "kernel",
		"processes", "disk", "diskio"}

	// Default output plugins
	outputDefaults = []string{"influxdb"}

	// envVarRe is a regex to find environment variables in the config file
	envVarRe = regexp.MustCompile(`\$\{(\w+)\}|\$(\w+)`)

	envVarEscaper = strings.NewReplacer(
		`"`, `\"`,
		`\`, `\\`,
	)
)

// Config specifies the URL/user/password for the database that telegraf
// will be logging to, as well as all the plugins that the user has
// specified
type Config struct {
	Tags          map[string]string
	InputFilters  []string
	OutputFilters []string

	Agent       *AgentConfig
	Inputs      []*models.RunningInput
	Outputs     []*models.RunningOutput
	Aggregators []*models.RunningAggregator
	// Processors have a slice wrapper type because they need to be sorted
	Processors    models.RunningProcessors
	AggProcessors models.RunningProcessors
}

func NewConfig() *Config {
	c := &Config{
		// Agent defaults:
		Agent: &AgentConfig{
			Interval:                   internal.Duration{Duration: 10 * time.Second},
			RoundInterval:              true,
			FlushInterval:              internal.Duration{Duration: 10 * time.Second},
			LogTarget:                  "file",
			LogfileRotationMaxArchives: 5,
		},

		Tags:          make(map[string]string),
		Inputs:        make([]*models.RunningInput, 0),
		Outputs:       make([]*models.RunningOutput, 0),
		Processors:    make([]*models.RunningProcessor, 0),
		AggProcessors: make([]*models.RunningProcessor, 0),
		InputFilters:  make([]string, 0),
		OutputFilters: make([]string, 0),
	}
	return c
}

type AgentConfig struct {
	// Interval at which to gather information
	Interval internal.Duration

	// RoundInterval rounds collection interval to 'interval'.
	//     ie, if Interval=10s then always collect on :00, :10, :20, etc.
	RoundInterval bool

	// By default or when set to "0s", precision will be set to the same
	// timestamp order as the collection interval, with the maximum being 1s.
	//   ie, when interval = "10s", precision will be "1s"
	//       when interval = "250ms", precision will be "1ms"
	// Precision will NOT be used for service inputs. It is up to each individual
	// service input to set the timestamp at the appropriate precision.
	Precision internal.Duration

	// CollectionJitter is used to jitter the collection by a random amount.
	// Each plugin will sleep for a random time within jitter before collecting.
	// This can be used to avoid many plugins querying things like sysfs at the
	// same time, which can have a measurable effect on the system.
	CollectionJitter internal.Duration

	// FlushInterval is the Interval at which to flush data
	FlushInterval internal.Duration

	// FlushJitter Jitters the flush interval by a random amount.
	// This is primarily to avoid large write spikes for users running a large
	// number of telegraf instances.
	// ie, a jitter of 5s and interval 10s means flushes will happen every 10-15s
	FlushJitter internal.Duration

	// MetricBatchSize is the maximum number of metrics that is wrote to an
	// output plugin in one call.
	MetricBatchSize int

	// MetricBufferLimit is the max number of metrics that each output plugin
	// will cache. The buffer is cleared when a successful write occurs. When
	// full, the oldest metrics will be overwritten. This number should be a
	// multiple of MetricBatchSize. Due to current implementation, this could
	// not be less than 2 times MetricBatchSize.
	MetricBufferLimit int

	// FlushBufferWhenFull tells Telegraf to flush the metric buffer whenever
	// it fills up, regardless of FlushInterval. Setting this option to true
	// does _not_ deactivate FlushInterval.
	FlushBufferWhenFull bool // deprecated in 0.13; has no effect

	// TODO(cam): Remove UTC and parameter, they are no longer
	// valid for the agent config. Leaving them here for now for backwards-
	// compatibility
	UTC bool `toml:"utc"` // deprecated in 1.0.0; has no effect

	// Debug is the option for running in debug mode
	Debug bool `toml:"debug"`

	// Quiet is the option for running in quiet mode
	Quiet bool `toml:"quiet"`

	// Log target controls the destination for logs and can be one of "file",
	// "stderr" or, on Windows, "eventlog".  When set to "file", the output file
	// is determined by the "logfile" setting.
	LogTarget string `toml:"logtarget"`

	// Name of the file to be logged to when using the "file" logtarget.  If set to
	// the empty string then logs are written to stderr.
	Logfile string `toml:"logfile"`

	// The file will be rotated after the time interval specified.  When set
	// to 0 no time based rotation is performed.
	LogfileRotationInterval internal.Duration `toml:"logfile_rotation_interval"`

	// The logfile will be rotated when it becomes larger than the specified
	// size.  When set to 0 no size based rotation is performed.
	LogfileRotationMaxSize internal.Size `toml:"logfile_rotation_max_size"`

	// Maximum number of rotated archives to keep, any older logs are deleted.
	// If set to -1, no archives are removed.
	LogfileRotationMaxArchives int `toml:"logfile_rotation_max_archives"`

	Hostname     string
	OmitHostname bool
}

// InputNames returns a list of strings of the configured inputs.
func (c *Config) InputNames() []string {
	var name []string
	for _, input := range c.Inputs {
		name = append(name, input.Config.Name)
	}
	return PluginNameCounts(name)
}

// AggregatorNames returns a list of strings of the configured aggregators.
func (c *Config) AggregatorNames() []string {
	var name []string
	for _, aggregator := range c.Aggregators {
		name = append(name, aggregator.Config.Name)
	}
	return PluginNameCounts(name)
}

// ProcessorNames returns a list of strings of the configured processors.
func (c *Config) ProcessorNames() []string {
	var name []string
	for _, processor := range c.Processors {
		name = append(name, processor.Config.Name)
	}
	return PluginNameCounts(name)
}

// OutputNames returns a list of strings of the configured outputs.
func (c *Config) OutputNames() []string {
	var name []string
	for _, output := range c.Outputs {
		name = append(name, output.Config.Name)
	}
	return PluginNameCounts(name)
}

// PluginNameCounts returns a list of sorted plugin names and their count
func PluginNameCounts(plugins []string) []string {
	names := make(map[string]int)
	for _, plugin := range plugins {
		names[plugin]++
	}

	var namecount []string
	for name, count := range names {
		if count == 1 {
			namecount = append(namecount, name)
		} else {
			namecount = append(namecount, fmt.Sprintf("%s (%dx)", name, count))
		}
	}

	sort.Strings(namecount)
	return namecount
}

// ListTags returns a string of tags specified in the config,
// line-protocol style
func (c *Config) ListTags() string {
	var tags []string

	for k, v := range c.Tags {
		tags = append(tags, fmt.Sprintf("%s=%s", k, v))
	}

	sort.Strings(tags)

	return strings.Join(tags, " ")
}

var header = `# Telegraf Configuration
#
# Telegraf is entirely plugin driven. All metrics are gathered from the
# declared inputs, and sent to the declared outputs.
#
# Plugins must be declared in here to be active.
# To deactivate a plugin, comment out the name and any variables.
#
# Use 'telegraf -config telegraf.conf -test' to see what metrics a config
# file would generate.
#
# Environment variables can be used anywhere in this config file, simply surround
# them with ${}. For strings the variable must be within quotes (ie, "${STR_VAR}"),
# for numbers and booleans they should be plain (ie, ${INT_VAR}, ${BOOL_VAR})

`
var globalTagsConfig = `
# Global tags can be specified here in key="value" format.
[global_tags]
  # dc = "us-east-1" # will tag all metrics with dc=us-east-1
  # rack = "1a"
  ## Environment variables can be used as tags, and throughout the config file
  # user = "$USER"

`
var agentConfig = `
# Configuration for telegraf agent
[agent]
  ## Default data collection interval for all inputs
  interval = "10s"
  ## Rounds collection interval to 'interval'
  ## ie, if interval="10s" then always collect on :00, :10, :20, etc.
  round_interval = true

  ## Telegraf will send metrics to outputs in batches of at most
  ## metric_batch_size metrics.
  ## This controls the size of writes that Telegraf sends to output plugins.
  metric_batch_size = 1000

  ## Maximum number of unwritten metrics per output.  Increasing this value
  ## allows for longer periods of output downtime without dropping metrics at the
  ## cost of higher maximum memory usage.
  metric_buffer_limit = 10000

  ## Collection jitter is used to jitter the collection by a random amount.
  ## Each plugin will sleep for a random time within jitter before collecting.
  ## This can be used to avoid many plugins querying things like sysfs at the
  ## same time, which can have a measurable effect on the system.
  collection_jitter = "0s"

  ## Default flushing interval for all outputs. Maximum flush_interval will be
  ## flush_interval + flush_jitter
  flush_interval = "10s"
  ## Jitter the flush interval by a random amount. This is primarily to avoid
  ## large write spikes for users running a large number of telegraf instances.
  ## ie, a jitter of 5s and interval 10s means flushes will happen every 10-15s
  flush_jitter = "0s"

  ## By default or when set to "0s", precision will be set to the same
  ## timestamp order as the collection interval, with the maximum being 1s.
  ##   ie, when interval = "10s", precision will be "1s"
  ##       when interval = "250ms", precision will be "1ms"
  ## Precision will NOT be used for service inputs. It is up to each individual
  ## service input to set the timestamp at the appropriate precision.
  ## Valid time units are "ns", "us" (or "µs"), "ms", "s".
  precision = ""

  ## Log at debug level.
  # debug = false
  ## Log only error level messages.
  # quiet = false

  ## Log target controls the destination for logs and can be one of "file",
  ## "stderr" or, on Windows, "eventlog".  When set to "file", the output file
  ## is determined by the "logfile" setting.
  # logtarget = "file"

  ## Name of the file to be logged to when using the "file" logtarget.  If set to
  ## the empty string then logs are written to stderr.
  # logfile = ""

  ## The logfile will be rotated after the time interval specified.  When set
  ## to 0 no time based rotation is performed.  Logs are rotated only when
  ## written to, if there is no log activity rotation may be delayed.
  # logfile_rotation_interval = "0d"

  ## The logfile will be rotated when it becomes larger than the specified
  ## size.  When set to 0 no size based rotation is performed.
  # logfile_rotation_max_size = "0MB"

  ## Maximum number of rotated archives to keep, any older logs are deleted.
  ## If set to -1, no archives are removed.
  # logfile_rotation_max_archives = 5

  ## Override default hostname, if empty use os.Hostname()
  hostname = ""
  ## If set to true, do no set the "host" tag in the telegraf agent.
  omit_hostname = false

`

var outputHeader = `
###############################################################################
#                            OUTPUT PLUGINS                                   #
###############################################################################

`

var processorHeader = `
###############################################################################
#                            PROCESSOR PLUGINS                                #
###############################################################################

`

var aggregatorHeader = `
###############################################################################
#                            AGGREGATOR PLUGINS                               #
###############################################################################

`

var inputHeader = `
###############################################################################
#                            INPUT PLUGINS                                    #
###############################################################################

`

var serviceInputHeader = `
###############################################################################
#                            SERVICE INPUT PLUGINS                            #
###############################################################################

`

// PrintSampleConfig prints the sample config
func PrintSampleConfig(
	sectionFilters []string,
	inputFilters []string,
	outputFilters []string,
	aggregatorFilters []string,
	processorFilters []string,
) {
	// print headers
	fmt.Printf(header)

	if len(sectionFilters) == 0 {
		sectionFilters = sectionDefaults
	}
	printFilteredGlobalSections(sectionFilters)

	// print output plugins
	if sliceContains("outputs", sectionFilters) {
		if len(outputFilters) != 0 {
			if len(outputFilters) >= 3 && outputFilters[1] != "none" {
				fmt.Printf(outputHeader)
			}
			printFilteredOutputs(outputFilters, false)
		} else {
			fmt.Printf(outputHeader)
			printFilteredOutputs(outputDefaults, false)
			// Print non-default outputs, commented
			var pnames []string
			for pname := range outputs.Outputs {
				if !sliceContains(pname, outputDefaults) {
					pnames = append(pnames, pname)
				}
			}
			sort.Strings(pnames)
			printFilteredOutputs(pnames, true)
		}
	}

	// print processor plugins
	if sliceContains("processors", sectionFilters) {
		if len(processorFilters) != 0 {
			if len(processorFilters) >= 3 && processorFilters[1] != "none" {
				fmt.Printf(processorHeader)
			}
			printFilteredProcessors(processorFilters, false)
		} else {
			fmt.Printf(processorHeader)
			pnames := []string{}
			for pname := range processors.Processors {
				pnames = append(pnames, pname)
			}
			sort.Strings(pnames)
			printFilteredProcessors(pnames, true)
		}
	}

	// print aggregator plugins
	if sliceContains("aggregators", sectionFilters) {
		if len(aggregatorFilters) != 0 {
			if len(aggregatorFilters) >= 3 && aggregatorFilters[1] != "none" {
				fmt.Printf(aggregatorHeader)
			}
			printFilteredAggregators(aggregatorFilters, false)
		} else {
			fmt.Printf(aggregatorHeader)
			pnames := []string{}
			for pname := range aggregators.Aggregators {
				pnames = append(pnames, pname)
			}
			sort.Strings(pnames)
			printFilteredAggregators(pnames, true)
		}
	}

	// print input plugins
	if sliceContains("inputs", sectionFilters) {
		if len(inputFilters) != 0 {
			if len(inputFilters) >= 3 && inputFilters[1] != "none" {
				fmt.Printf(inputHeader)
			}
			printFilteredInputs(inputFilters, false)
		} else {
			fmt.Printf(inputHeader)
			printFilteredInputs(inputDefaults, false)
			// Print non-default inputs, commented
			var pnames []string
			for pname := range inputs.Inputs {
				if !sliceContains(pname, inputDefaults) {
					pnames = append(pnames, pname)
				}
			}
			sort.Strings(pnames)
			printFilteredInputs(pnames, true)
		}
	}
}

func printFilteredProcessors(processorFilters []string, commented bool) {
	// Filter processors
	var pnames []string
	for pname := range processors.Processors {
		if sliceContains(pname, processorFilters) {
			pnames = append(pnames, pname)
		}
	}
	sort.Strings(pnames)

	// Print Outputs
	for _, pname := range pnames {
		creator := processors.Processors[pname]
		output := creator()
		printConfig(pname, output, "processors", commented)
	}
}

func printFilteredAggregators(aggregatorFilters []string, commented bool) {
	// Filter outputs
	var anames []string
	for aname := range aggregators.Aggregators {
		if sliceContains(aname, aggregatorFilters) {
			anames = append(anames, aname)
		}
	}
	sort.Strings(anames)

	// Print Outputs
	for _, aname := range anames {
		creator := aggregators.Aggregators[aname]
		output := creator()
		printConfig(aname, output, "aggregators", commented)
	}
}

func printFilteredInputs(inputFilters []string, commented bool) {
	// Filter inputs
	var pnames []string
	for pname := range inputs.Inputs {
		if sliceContains(pname, inputFilters) {
			pnames = append(pnames, pname)
		}
	}
	sort.Strings(pnames)

	// cache service inputs to print them at the end
	servInputs := make(map[string]telegraf.ServiceInput)
	// for alphabetical looping:
	servInputNames := []string{}

	// Print Inputs
	for _, pname := range pnames {
		if pname == "cisco_telemetry_gnmi" {
			continue
		}
		creator := inputs.Inputs[pname]
		input := creator()

		switch p := input.(type) {
		case telegraf.ServiceInput:
			servInputs[pname] = p
			servInputNames = append(servInputNames, pname)
			continue
		}

		printConfig(pname, input, "inputs", commented)
	}

	// Print Service Inputs
	if len(servInputs) == 0 {
		return
	}
	sort.Strings(servInputNames)

	fmt.Printf(serviceInputHeader)
	for _, name := range servInputNames {
		printConfig(name, servInputs[name], "inputs", commented)
	}
}

func printFilteredOutputs(outputFilters []string, commented bool) {
	// Filter outputs
	var onames []string
	for oname := range outputs.Outputs {
		if sliceContains(oname, outputFilters) {
			onames = append(onames, oname)
		}
	}
	sort.Strings(onames)

	// Print Outputs
	for _, oname := range onames {
		creator := outputs.Outputs[oname]
		output := creator()
		printConfig(oname, output, "outputs", commented)
	}
}

func printFilteredGlobalSections(sectionFilters []string) {
	if sliceContains("global_tags", sectionFilters) {
		fmt.Printf(globalTagsConfig)
	}

	if sliceContains("agent", sectionFilters) {
		fmt.Printf(agentConfig)
	}
}

func printConfig(name string, p telegraf.PluginDescriber, op string, commented bool) {
	comment := ""
	if commented {
		comment = "# "
	}
	fmt.Printf("\n%s# %s\n%s[[%s.%s]]", comment, p.Description(), comment,
		op, name)

	config := p.SampleConfig()
	if config == "" {
		fmt.Printf("\n%s  # no configuration\n\n", comment)
	} else {
		lines := strings.Split(config, "\n")
		for i, line := range lines {
			if i == 0 || i == len(lines)-1 {
				fmt.Print("\n")
				continue
			}
			fmt.Print(strings.TrimRight(comment+line, " ") + "\n")
		}
	}
}

func sliceContains(name string, list []string) bool {
	for _, b := range list {
		if b == name {
			return true
		}
	}
	return false
}

// PrintInputConfig prints the config usage of a single input.
func PrintInputConfig(name string) error {
	if creator, ok := inputs.Inputs[name]; ok {
		printConfig(name, creator(), "inputs", false)
	} else {
		return errors.New(fmt.Sprintf("Input %s not found", name))
	}
	return nil
}

// PrintOutputConfig prints the config usage of a single output.
func PrintOutputConfig(name string) error {
	if creator, ok := outputs.Outputs[name]; ok {
		printConfig(name, creator(), "outputs", false)
	} else {
		return errors.New(fmt.Sprintf("Output %s not found", name))
	}
	return nil
}

func (c *Config) LoadDirectory(path string) error {
	walkfn := func(thispath string, info os.FileInfo, _ error) error {
		if info == nil {
			log.Printf("W! Telegraf is not permitted to read %s", thispath)
			return nil
		}

		if info.IsDir() {
			if strings.HasPrefix(info.Name(), "..") {
				// skip Kubernetes mounts, prevening loading the same config twice
				return filepath.SkipDir
			}

			return nil
		}
		name := info.Name()
		if len(name) < 6 || name[len(name)-5:] != ".conf" {
			return nil
		}
		err := c.LoadConfig(thispath)
		if err != nil {
			return err
		}
		return nil
	}
	return filepath.Walk(path, walkfn)
}

// Try to find a default config file at these locations (in order):
//  1. $TELEGRAF_CONFIG_PATH
//  2. $HOME/.telegraf/telegraf.conf
//  3. /etc/telegraf/telegraf.conf
func getDefaultConfigPath() (string, error) {
	envfile := os.Getenv("TELEGRAF_CONFIG_PATH")
	homefile := os.ExpandEnv("${HOME}/.telegraf/telegraf.conf")
	etcfile := "/etc/telegraf/telegraf.conf"
	if runtime.GOOS == "windows" {
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" { // Should never happen
			programFiles = `C:\Program Files`
		}
		etcfile = programFiles + `\Telegraf\telegraf.conf`
	}
	for _, path := range []string{envfile, homefile, etcfile} {
		if _, err := os.Stat(path); err == nil {
			log.Printf("I! Using config file: %s", path)
			return path, nil
		}
	}

	// if we got here, we didn't find a file in a default location
	return "", fmt.Errorf("No config file specified, and could not find one"+
		" in $TELEGRAF_CONFIG_PATH, %s, or %s", homefile, etcfile)
}

// LoadConfig loads the given config file and applies it to c
func (c *Config) LoadConfig(path string) error {
	var err error
	if path == "" {
		if path, err = getDefaultConfigPath(); err != nil {
			return err
		}
	}
	data, err := loadConfig(path)
	if err != nil {
		return fmt.Errorf("Error loading config file %s: %w", path, err)
	}

	if err = c.LoadConfigData(data); err != nil {
		return fmt.Errorf("Error loading config file %s: %w", path, err)
	}
	return nil
}

// LoadConfigData loads TOML-formatted config data
func (c *Config) LoadConfigData(data []byte) error {
	tbl, err := parseConfig(data)
	if err != nil {
		return fmt.Errorf("Error parsing data: %s", err)
	}

	// Parse tags tables first:
	for _, tableName := range []string{"tags", "global_tags"} {
		if val, ok := tbl.Fields[tableName]; ok {
			subTable, ok := val.(*ast.Table)
			if !ok {
				return fmt.Errorf("invalid configuration, bad table name %q", tableName)
			}
			if err = toml.UnmarshalTable(subTable, c.Tags); err != nil {
				return fmt.Errorf("error parsing table name %q: %w", tableName, err)
			}
		}
	}

	// Parse agent table:
	if val, ok := tbl.Fields["agent"]; ok {
		subTable, ok := val.(*ast.Table)
		if !ok {
			return fmt.Errorf("invalid configuration, error parsing agent table")
		}
		if err = toml.UnmarshalTable(subTable, c.Agent); err != nil {
			return fmt.Errorf("error parsing agent table: %w", err)
		}
	}

	if !c.Agent.OmitHostname {
		if c.Agent.Hostname == "" {
			hostname, err := os.Hostname()
			if err != nil {
				return err
			}

			c.Agent.Hostname = hostname
		}

		c.Tags["host"] = c.Agent.Hostname
	}

	// Parse all the rest of the plugins:
	for name, val := range tbl.Fields {
		subTable, ok := val.(*ast.Table)
		if !ok {
			return fmt.Errorf("invalid configuration, error parsing field %q as table", name)
		}

		switch name {
		case "agent", "global_tags", "tags":
		case "outputs":
			for pluginName, pluginVal := range subTable.Fields {
				switch pluginSubTable := pluginVal.(type) {
				// legacy [outputs.influxdb] support
				case *ast.Table:
					if err = c.addOutput(pluginName, pluginSubTable); err != nil {
						return fmt.Errorf("Error parsing %s, %s", pluginName, err)
					}
				case []*ast.Table:
					for _, t := range pluginSubTable {
						if err = c.addOutput(pluginName, t); err != nil {
							return fmt.Errorf("Error parsing %s array, %s", pluginName, err)
						}
					}
				default:
					return fmt.Errorf("Unsupported config format: %s",
						pluginName)
				}
			}
		case "inputs", "plugins":
			for pluginName, pluginVal := range subTable.Fields {
				switch pluginSubTable := pluginVal.(type) {
				// legacy [inputs.cpu] support
				case *ast.Table:
					if err = c.addInput(pluginName, pluginSubTable); err != nil {
						return fmt.Errorf("Error parsing %s, %s", pluginName, err)
					}
				case []*ast.Table:
					for _, t := range pluginSubTable {
						if err = c.addInput(pluginName, t); err != nil {
							return fmt.Errorf("Error parsing %s, %s", pluginName, err)
						}
					}
				default:
					return fmt.Errorf("Unsupported config format: %s",
						pluginName)
				}
			}
		case "processors":
			for pluginName, pluginVal := range subTable.Fields {
				switch pluginSubTable := pluginVal.(type) {
				case []*ast.Table:
					for _, t := range pluginSubTable {
						if err = c.addProcessor(pluginName, t); err != nil {
							return fmt.Errorf("Error parsing %s, %s", pluginName, err)
						}
					}
				default:
					return fmt.Errorf("Unsupported config format: %s",
						pluginName)
				}
			}
		case "aggregators":
			for pluginName, pluginVal := range subTable.Fields {
				switch pluginSubTable := pluginVal.(type) {
				case []*ast.Table:
					for _, t := range pluginSubTable {
						if err = c.addAggregator(pluginName, t); err != nil {
							return fmt.Errorf("Error parsing %s, %s", pluginName, err)
						}
					}
				default:
					return fmt.Errorf("Unsupported config format: %s",
						pluginName)
				}
			}
		// Assume it's an input input for legacy config file support if no other
		// identifiers are present
		default:
			if err = c.addInput(name, subTable); err != nil {
				return fmt.Errorf("Error parsing %s, %s", name, err)
			}
		}
	}

	if len(c.Processors) > 1 {
		sort.Sort(c.Processors)
	}

	return nil
}

// trimBOM trims the Byte-Order-Marks from the beginning of the file.
// this is for Windows compatibility only.
// see https://github.com/shanas-swi/telegraf-v1.16.3/issues/1378
func trimBOM(f []byte) []byte {
	return bytes.TrimPrefix(f, []byte("\xef\xbb\xbf"))
}

// escapeEnv escapes a value for inserting into a TOML string.
func escapeEnv(value string) string {
	return envVarEscaper.Replace(value)
}

func loadConfig(config string) ([]byte, error) {
	u, err := url.Parse(config)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "https", "http":
		return fetchConfig(u)
	default:
		// If it isn't a https scheme, try it as a file.
	}
	return ioutil.ReadFile(config)

}

func fetchConfig(u *url.URL) ([]byte, error) {
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	if v, exists := os.LookupEnv("INFLUX_TOKEN"); exists {
		req.Header.Add("Authorization", "Token "+v)
	}
	req.Header.Add("Accept", "application/toml")
	req.Header.Set("User-Agent", internal.ProductToken())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to retrieve remote config: %s", resp.Status)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// parseConfig loads a TOML configuration from a provided path and
// returns the AST produced from the TOML parser. When loading the file, it
// will find environment variables and replace them.
func parseConfig(contents []byte) (*ast.Table, error) {
	contents = trimBOM(contents)

	parameters := envVarRe.FindAllSubmatch(contents, -1)
	for _, parameter := range parameters {
		if len(parameter) != 3 {
			continue
		}

		var env_var []byte
		if parameter[1] != nil {
			env_var = parameter[1]
		} else if parameter[2] != nil {
			env_var = parameter[2]
		} else {
			continue
		}

		env_val, ok := os.LookupEnv(strings.TrimPrefix(string(env_var), "$"))
		if ok {
			env_val = escapeEnv(env_val)
			contents = bytes.Replace(contents, parameter[0], []byte(env_val), 1)
		}
	}

	return toml.Parse(contents)
}

func (c *Config) addAggregator(name string, table *ast.Table) error {
	creator, ok := aggregators.Aggregators[name]
	if !ok {
		return fmt.Errorf("Undefined but requested aggregator: %s", name)
	}
	aggregator := creator()

	conf, err := buildAggregator(name, table)
	if err != nil {
		return err
	}

	if err := toml.UnmarshalTable(table, aggregator); err != nil {
		return err
	}

	c.Aggregators = append(c.Aggregators, models.NewRunningAggregator(aggregator, conf))
	return nil
}

func (c *Config) addProcessor(name string, table *ast.Table) error {
	creator, ok := processors.Processors[name]
	if !ok {
		return fmt.Errorf("Undefined but requested processor: %s", name)
	}

	processorConfig, err := buildProcessor(name, table)
	if err != nil {
		return err
	}

	rf, err := c.newRunningProcessor(creator, processorConfig, name, table)
	if err != nil {
		return err
	}
	c.Processors = append(c.Processors, rf)

	// save a copy for the aggregator
	rf, err = c.newRunningProcessor(creator, processorConfig, name, table)
	if err != nil {
		return err
	}
	c.AggProcessors = append(c.AggProcessors, rf)

	return nil
}

func (c *Config) newRunningProcessor(
	creator processors.StreamingCreator,
	processorConfig *models.ProcessorConfig,
	name string,
	table *ast.Table,
) (*models.RunningProcessor, error) {
	processor := creator()

	if p, ok := processor.(unwrappable); ok {
		if err := toml.UnmarshalTable(table, p.Unwrap()); err != nil {
			return nil, err
		}
	} else {
		if err := toml.UnmarshalTable(table, processor); err != nil {
			return nil, err
		}
	}

	rf := models.NewRunningProcessor(processor, processorConfig)
	return rf, nil
}

func (c *Config) addOutput(name string, table *ast.Table) error {
	if len(c.OutputFilters) > 0 && !sliceContains(name, c.OutputFilters) {
		return nil
	}
	creator, ok := outputs.Outputs[name]
	if !ok {
		return fmt.Errorf("Undefined but requested output: %s", name)
	}
	output := creator()

	// If the output has a SetSerializer function, then this means it can write
	// arbitrary types of output, so build the serializer and set it.
	switch t := output.(type) {
	case serializers.SerializerOutput:
		serializer, err := buildSerializer(name, table)
		if err != nil {
			return err
		}
		t.SetSerializer(serializer)
	}

	outputConfig, err := buildOutput(name, table)
	if err != nil {
		return err
	}

	if err := toml.UnmarshalTable(table, output); err != nil {
		return err
	}

	ro := models.NewRunningOutput(name, output, outputConfig,
		c.Agent.MetricBatchSize, c.Agent.MetricBufferLimit)
	c.Outputs = append(c.Outputs, ro)
	return nil
}

func (c *Config) addInput(name string, table *ast.Table) error {
	if len(c.InputFilters) > 0 && !sliceContains(name, c.InputFilters) {
		return nil
	}
	// Legacy support renaming io input to diskio
	if name == "io" {
		name = "diskio"
	}

	creator, ok := inputs.Inputs[name]
	if !ok {
		return fmt.Errorf("Undefined but requested input: %s", name)
	}
	input := creator()

	// If the input has a SetParser function, then this means it can accept
	// arbitrary types of input, so build the parser and set it.
	if t, ok := input.(parsers.ParserInput); ok {
		parser, err := buildParser(name, table)
		if err != nil {
			return err
		}
		t.SetParser(parser)
	}

	if t, ok := input.(parsers.ParserFuncInput); ok {
		config, err := getParserConfig(name, table)
		if err != nil {
			return err
		}
		t.SetParserFunc(func() (parsers.Parser, error) {
			return parsers.NewParser(config)
		})
	}

	pluginConfig, err := buildInput(name, table)
	if err != nil {
		return err
	}

	if err := toml.UnmarshalTable(table, input); err != nil {
		return err
	}

	rp := models.NewRunningInput(input, pluginConfig)
	rp.SetDefaultTags(c.Tags)
	c.Inputs = append(c.Inputs, rp)
	return nil
}

// buildAggregator parses Aggregator specific items from the ast.Table,
// builds the filter and returns a
// models.AggregatorConfig to be inserted into models.RunningAggregator
func buildAggregator(name string, tbl *ast.Table) (*models.AggregatorConfig, error) {
	conf := &models.AggregatorConfig{
		Name:   name,
		Delay:  time.Millisecond * 100,
		Period: time.Second * 30,
		Grace:  time.Second * 0,
	}

	if err := getConfigDuration(tbl, "period", &conf.Period); err != nil {
		return nil, err
	}

	if err := getConfigDuration(tbl, "delay", &conf.Delay); err != nil {
		return nil, err
	}

	if err := getConfigDuration(tbl, "grace", &conf.Grace); err != nil {
		return nil, err
	}

	if node, ok := tbl.Fields["drop_original"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				conf.DropOriginal, err = strconv.ParseBool(b.Value)
				if err != nil {
					return nil, fmt.Errorf("error parsing boolean value for %s: %s", name, err)
				}
			}
		}
	}

	if node, ok := tbl.Fields["name_prefix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				conf.MeasurementPrefix = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_suffix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				conf.MeasurementSuffix = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_override"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				conf.NameOverride = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["alias"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				conf.Alias = str.Value
			}
		}
	}

	conf.Tags = make(map[string]string)
	if node, ok := tbl.Fields["tags"]; ok {
		if subtbl, ok := node.(*ast.Table); ok {
			if err := toml.UnmarshalTable(subtbl, conf.Tags); err != nil {
				return nil, fmt.Errorf("could not parse tags for input %s", name)
			}
		}
	}

	delete(tbl.Fields, "drop_original")
	delete(tbl.Fields, "name_prefix")
	delete(tbl.Fields, "name_suffix")
	delete(tbl.Fields, "name_override")
	delete(tbl.Fields, "alias")
	delete(tbl.Fields, "tags")
	var err error
	conf.Filter, err = buildFilter(tbl)
	if err != nil {
		return conf, err
	}
	return conf, nil
}

// buildProcessor parses Processor specific items from the ast.Table,
// builds the filter and returns a
// models.ProcessorConfig to be inserted into models.RunningProcessor
func buildProcessor(name string, tbl *ast.Table) (*models.ProcessorConfig, error) {
	conf := &models.ProcessorConfig{Name: name}

	if node, ok := tbl.Fields["order"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Integer); ok {
				var err error
				conf.Order, err = strconv.ParseInt(b.Value, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("error parsing int value for %s: %s", name, err)
				}
			}
		}
	}

	if node, ok := tbl.Fields["alias"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				conf.Alias = str.Value
			}
		}
	}

	delete(tbl.Fields, "alias")
	delete(tbl.Fields, "order")
	var err error
	conf.Filter, err = buildFilter(tbl)
	if err != nil {
		return conf, err
	}
	return conf, nil
}

// buildFilter builds a Filter
// (tagpass/tagdrop/namepass/namedrop/fieldpass/fielddrop) to
// be inserted into the models.OutputConfig/models.InputConfig
// to be used for glob filtering on tags and measurements
func buildFilter(tbl *ast.Table) (models.Filter, error) {
	f := models.Filter{}

	if node, ok := tbl.Fields["namepass"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						f.NamePass = append(f.NamePass, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["namedrop"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						f.NameDrop = append(f.NameDrop, str.Value)
					}
				}
			}
		}
	}

	fields := []string{"pass", "fieldpass"}
	for _, field := range fields {
		if node, ok := tbl.Fields[field]; ok {
			if kv, ok := node.(*ast.KeyValue); ok {
				if ary, ok := kv.Value.(*ast.Array); ok {
					for _, elem := range ary.Value {
						if str, ok := elem.(*ast.String); ok {
							f.FieldPass = append(f.FieldPass, str.Value)
						}
					}
				}
			}
		}
	}

	fields = []string{"drop", "fielddrop"}
	for _, field := range fields {
		if node, ok := tbl.Fields[field]; ok {
			if kv, ok := node.(*ast.KeyValue); ok {
				if ary, ok := kv.Value.(*ast.Array); ok {
					for _, elem := range ary.Value {
						if str, ok := elem.(*ast.String); ok {
							f.FieldDrop = append(f.FieldDrop, str.Value)
						}
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["tagpass"]; ok {
		if subtbl, ok := node.(*ast.Table); ok {
			for name, val := range subtbl.Fields {
				if kv, ok := val.(*ast.KeyValue); ok {
					tagfilter := &models.TagFilter{Name: name}
					if ary, ok := kv.Value.(*ast.Array); ok {
						for _, elem := range ary.Value {
							if str, ok := elem.(*ast.String); ok {
								tagfilter.Filter = append(tagfilter.Filter, str.Value)
							}
						}
					}
					f.TagPass = append(f.TagPass, *tagfilter)
				}
			}
		}
	}

	if node, ok := tbl.Fields["tagdrop"]; ok {
		if subtbl, ok := node.(*ast.Table); ok {
			for name, val := range subtbl.Fields {
				if kv, ok := val.(*ast.KeyValue); ok {
					tagfilter := &models.TagFilter{Name: name}
					if ary, ok := kv.Value.(*ast.Array); ok {
						for _, elem := range ary.Value {
							if str, ok := elem.(*ast.String); ok {
								tagfilter.Filter = append(tagfilter.Filter, str.Value)
							}
						}
					}
					f.TagDrop = append(f.TagDrop, *tagfilter)
				}
			}
		}
	}

	if node, ok := tbl.Fields["tagexclude"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						f.TagExclude = append(f.TagExclude, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["taginclude"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						f.TagInclude = append(f.TagInclude, str.Value)
					}
				}
			}
		}
	}
	if err := f.Compile(); err != nil {
		return f, err
	}

	delete(tbl.Fields, "namedrop")
	delete(tbl.Fields, "namepass")
	delete(tbl.Fields, "fielddrop")
	delete(tbl.Fields, "fieldpass")
	delete(tbl.Fields, "drop")
	delete(tbl.Fields, "pass")
	delete(tbl.Fields, "tagdrop")
	delete(tbl.Fields, "tagpass")
	delete(tbl.Fields, "tagexclude")
	delete(tbl.Fields, "taginclude")
	return f, nil
}

// buildInput parses input specific items from the ast.Table,
// builds the filter and returns a
// models.InputConfig to be inserted into models.RunningInput
func buildInput(name string, tbl *ast.Table) (*models.InputConfig, error) {
	cp := &models.InputConfig{Name: name}

	if err := getConfigDuration(tbl, "interval", &cp.Interval); err != nil {
		return nil, err
	}

	if err := getConfigDuration(tbl, "precision", &cp.Precision); err != nil {
		return nil, err
	}

	if err := getConfigDuration(tbl, "collection_jitter", &cp.CollectionJitter); err != nil {
		return nil, err
	}

	if node, ok := tbl.Fields["name_prefix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				cp.MeasurementPrefix = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_suffix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				cp.MeasurementSuffix = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_override"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				cp.NameOverride = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["alias"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				cp.Alias = str.Value
			}
		}
	}

	cp.Tags = make(map[string]string)
	if node, ok := tbl.Fields["tags"]; ok {
		if subtbl, ok := node.(*ast.Table); ok {
			if err := toml.UnmarshalTable(subtbl, cp.Tags); err != nil {
				return nil, fmt.Errorf("could not parse tags for input %s\n", name)
			}
		}
	}

	delete(tbl.Fields, "name_prefix")
	delete(tbl.Fields, "name_suffix")
	delete(tbl.Fields, "name_override")
	delete(tbl.Fields, "alias")
	delete(tbl.Fields, "tags")
	var err error
	cp.Filter, err = buildFilter(tbl)
	if err != nil {
		return cp, err
	}
	return cp, nil
}

// buildParser grabs the necessary entries from the ast.Table for creating
// a parsers.Parser object, and creates it, which can then be added onto
// an Input object.
func buildParser(name string, tbl *ast.Table) (parsers.Parser, error) {
	config, err := getParserConfig(name, tbl)
	if err != nil {
		return nil, err
	}
	return parsers.NewParser(config)
}

func getParserConfig(name string, tbl *ast.Table) (*parsers.Config, error) {
	c := &parsers.Config{
		JSONStrict: true,
	}

	if node, ok := tbl.Fields["data_format"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DataFormat = str.Value
			}
		}
	}

	// Legacy support, exec plugin originally parsed JSON by default.
	if name == "exec" && c.DataFormat == "" {
		c.DataFormat = "json"
	} else if c.DataFormat == "" {
		c.DataFormat = "influx"
	}

	if node, ok := tbl.Fields["separator"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.Separator = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["templates"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.Templates = append(c.Templates, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["tag_keys"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.TagKeys = append(c.TagKeys, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["json_string_fields"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.JSONStringFields = append(c.JSONStringFields, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["json_name_key"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.JSONNameKey = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["json_query"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.JSONQuery = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["json_time_key"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.JSONTimeKey = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["json_time_format"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.JSONTimeFormat = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["json_timezone"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.JSONTimezone = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["json_strict"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.JSONStrict, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["data_type"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DataType = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["collectd_auth_file"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CollectdAuthFile = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["collectd_security_level"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CollectdSecurityLevel = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["collectd_parse_multivalue"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CollectdSplit = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["collectd_typesdb"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.CollectdTypesDB = append(c.CollectdTypesDB, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["dropwizard_metric_registry_path"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DropwizardMetricRegistryPath = str.Value
			}
		}
	}
	if node, ok := tbl.Fields["dropwizard_time_path"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DropwizardTimePath = str.Value
			}
		}
	}
	if node, ok := tbl.Fields["dropwizard_time_format"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DropwizardTimeFormat = str.Value
			}
		}
	}
	if node, ok := tbl.Fields["dropwizard_tags_path"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DropwizardTagsPath = str.Value
			}
		}
	}
	c.DropwizardTagPathsMap = make(map[string]string)
	if node, ok := tbl.Fields["dropwizard_tag_paths"]; ok {
		if subtbl, ok := node.(*ast.Table); ok {
			for name, val := range subtbl.Fields {
				if kv, ok := val.(*ast.KeyValue); ok {
					if str, ok := kv.Value.(*ast.String); ok {
						c.DropwizardTagPathsMap[name] = str.Value
					}
				}
			}
		}
	}

	//for grok data_format
	if node, ok := tbl.Fields["grok_named_patterns"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.GrokNamedPatterns = append(c.GrokNamedPatterns, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["grok_patterns"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.GrokPatterns = append(c.GrokPatterns, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["grok_custom_patterns"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.GrokCustomPatterns = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["grok_custom_pattern_files"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.GrokCustomPatternFiles = append(c.GrokCustomPatternFiles, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["grok_timezone"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.GrokTimezone = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["grok_unique_timestamp"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.GrokUniqueTimestamp = str.Value
			}
		}
	}

	//for csv parser
	if node, ok := tbl.Fields["csv_column_names"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.CSVColumnNames = append(c.CSVColumnNames, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["csv_column_types"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.CSVColumnTypes = append(c.CSVColumnTypes, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["csv_tag_columns"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.CSVTagColumns = append(c.CSVTagColumns, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["csv_delimiter"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CSVDelimiter = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["csv_comment"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CSVComment = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["csv_measurement_column"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CSVMeasurementColumn = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["csv_timestamp_column"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CSVTimestampColumn = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["csv_timestamp_format"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CSVTimestampFormat = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["csv_timezone"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.CSVTimezone = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["csv_header_row_count"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if integer, ok := kv.Value.(*ast.Integer); ok {
				v, err := integer.Int()
				if err != nil {
					return nil, err
				}
				c.CSVHeaderRowCount = int(v)
			}
		}
	}

	if node, ok := tbl.Fields["csv_skip_rows"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if integer, ok := kv.Value.(*ast.Integer); ok {
				v, err := integer.Int()
				if err != nil {
					return nil, err
				}
				c.CSVSkipRows = int(v)
			}
		}
	}

	if node, ok := tbl.Fields["csv_skip_columns"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if integer, ok := kv.Value.(*ast.Integer); ok {
				v, err := integer.Int()
				if err != nil {
					return nil, err
				}
				c.CSVSkipColumns = int(v)
			}
		}
	}

	if node, ok := tbl.Fields["csv_trim_space"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.Boolean); ok {
				//for config with no quotes
				val, err := strconv.ParseBool(str.Value)
				c.CSVTrimSpace = val
				if err != nil {
					return nil, fmt.Errorf("E! parsing to bool: %v", err)
				}
			}
		}
	}

	if node, ok := tbl.Fields["form_urlencoded_tag_keys"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.FormUrlencodedTagKeys = append(c.FormUrlencodedTagKeys, str.Value)
					}
				}
			}
		}
	}

	c.MetricName = name

	delete(tbl.Fields, "data_format")
	delete(tbl.Fields, "separator")
	delete(tbl.Fields, "templates")
	delete(tbl.Fields, "tag_keys")
	delete(tbl.Fields, "json_name_key")
	delete(tbl.Fields, "json_query")
	delete(tbl.Fields, "json_string_fields")
	delete(tbl.Fields, "json_time_format")
	delete(tbl.Fields, "json_time_key")
	delete(tbl.Fields, "json_timezone")
	delete(tbl.Fields, "json_strict")
	delete(tbl.Fields, "data_type")
	delete(tbl.Fields, "collectd_auth_file")
	delete(tbl.Fields, "collectd_security_level")
	delete(tbl.Fields, "collectd_typesdb")
	delete(tbl.Fields, "collectd_parse_multivalue")
	delete(tbl.Fields, "dropwizard_metric_registry_path")
	delete(tbl.Fields, "dropwizard_time_path")
	delete(tbl.Fields, "dropwizard_time_format")
	delete(tbl.Fields, "dropwizard_tags_path")
	delete(tbl.Fields, "dropwizard_tag_paths")
	delete(tbl.Fields, "grok_named_patterns")
	delete(tbl.Fields, "grok_patterns")
	delete(tbl.Fields, "grok_custom_patterns")
	delete(tbl.Fields, "grok_custom_pattern_files")
	delete(tbl.Fields, "grok_timezone")
	delete(tbl.Fields, "grok_unique_timestamp")
	delete(tbl.Fields, "csv_column_names")
	delete(tbl.Fields, "csv_column_types")
	delete(tbl.Fields, "csv_comment")
	delete(tbl.Fields, "csv_delimiter")
	delete(tbl.Fields, "csv_field_columns")
	delete(tbl.Fields, "csv_header_row_count")
	delete(tbl.Fields, "csv_measurement_column")
	delete(tbl.Fields, "csv_skip_columns")
	delete(tbl.Fields, "csv_skip_rows")
	delete(tbl.Fields, "csv_tag_columns")
	delete(tbl.Fields, "csv_timestamp_column")
	delete(tbl.Fields, "csv_timestamp_format")
	delete(tbl.Fields, "csv_timezone")
	delete(tbl.Fields, "csv_trim_space")
	delete(tbl.Fields, "form_urlencoded_tag_keys")

	return c, nil
}

// buildSerializer grabs the necessary entries from the ast.Table for creating
// a serializers.Serializer object, and creates it, which can then be added onto
// an Output object.
func buildSerializer(name string, tbl *ast.Table) (serializers.Serializer, error) {
	c := &serializers.Config{TimestampUnits: time.Duration(1 * time.Second)}

	if node, ok := tbl.Fields["data_format"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.DataFormat = str.Value
			}
		}
	}

	if c.DataFormat == "" {
		c.DataFormat = "influx"
	}

	if node, ok := tbl.Fields["prefix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.Prefix = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["template"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.Template = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["templates"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.Templates = append(c.Templates, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["carbon2_format"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.Carbon2Format = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["influx_max_line_bytes"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if integer, ok := kv.Value.(*ast.Integer); ok {
				v, err := integer.Int()
				if err != nil {
					return nil, err
				}
				c.InfluxMaxLineBytes = int(v)
			}
		}
	}

	if node, ok := tbl.Fields["influx_sort_fields"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.InfluxSortFields, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["influx_uint_support"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.InfluxUintSupport, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["graphite_tag_support"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.GraphiteTagSupport, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["graphite_separator"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				c.GraphiteSeparator = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["json_timestamp_units"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				timestampVal, err := time.ParseDuration(str.Value)
				if err != nil {
					return nil, fmt.Errorf("Unable to parse json_timestamp_units as a duration, %s", err)
				}
				// now that we have a duration, truncate it to the nearest
				// power of ten (just in case)
				nearest_exponent := int64(math.Log10(float64(timestampVal.Nanoseconds())))
				new_nanoseconds := int64(math.Pow(10.0, float64(nearest_exponent)))
				c.TimestampUnits = time.Duration(new_nanoseconds)
			}
		}
	}

	if node, ok := tbl.Fields["splunkmetric_hec_routing"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.HecRouting, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["splunkmetric_multimetric"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.SplunkmetricMultiMetric, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["wavefront_source_override"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if ary, ok := kv.Value.(*ast.Array); ok {
				for _, elem := range ary.Value {
					if str, ok := elem.(*ast.String); ok {
						c.WavefrontSourceOverride = append(c.WavefrontSourceOverride, str.Value)
					}
				}
			}
		}
	}

	if node, ok := tbl.Fields["wavefront_use_strict"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.WavefrontUseStrict, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["prometheus_export_timestamp"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.PrometheusExportTimestamp, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["prometheus_sort_metrics"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.PrometheusSortMetrics, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	if node, ok := tbl.Fields["prometheus_string_as_label"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if b, ok := kv.Value.(*ast.Boolean); ok {
				var err error
				c.PrometheusStringAsLabel, err = b.Boolean()
				if err != nil {
					return nil, err
				}
			}
		}
	}

	delete(tbl.Fields, "carbon2_format")
	delete(tbl.Fields, "influx_max_line_bytes")
	delete(tbl.Fields, "influx_sort_fields")
	delete(tbl.Fields, "influx_uint_support")
	delete(tbl.Fields, "graphite_tag_support")
	delete(tbl.Fields, "graphite_separator")
	delete(tbl.Fields, "data_format")
	delete(tbl.Fields, "prefix")
	delete(tbl.Fields, "template")
	delete(tbl.Fields, "templates")
	delete(tbl.Fields, "json_timestamp_units")
	delete(tbl.Fields, "splunkmetric_hec_routing")
	delete(tbl.Fields, "splunkmetric_multimetric")
	delete(tbl.Fields, "wavefront_source_override")
	delete(tbl.Fields, "wavefront_use_strict")
	delete(tbl.Fields, "prometheus_export_timestamp")
	delete(tbl.Fields, "prometheus_sort_metrics")
	delete(tbl.Fields, "prometheus_string_as_label")
	return serializers.NewSerializer(c)
}

// buildOutput parses output specific items from the ast.Table,
// builds the filter and returns an
// models.OutputConfig to be inserted into models.RunningInput
// Note: error exists in the return for future calls that might require error
func buildOutput(name string, tbl *ast.Table) (*models.OutputConfig, error) {
	filter, err := buildFilter(tbl)
	if err != nil {
		return nil, err
	}
	oc := &models.OutputConfig{
		Name:   name,
		Filter: filter,
	}

	// TODO
	// Outputs don't support FieldDrop/FieldPass, so set to NameDrop/NamePass
	if len(oc.Filter.FieldDrop) > 0 {
		oc.Filter.NameDrop = oc.Filter.FieldDrop
	}
	if len(oc.Filter.FieldPass) > 0 {
		oc.Filter.NamePass = oc.Filter.FieldPass
	}

	if err := getConfigDuration(tbl, "flush_interval", &oc.FlushInterval); err != nil {
		return nil, err
	}

	if err := getConfigDuration(tbl, "flush_jitter", &oc.FlushJitter); err != nil {
		return nil, err
	}

	if node, ok := tbl.Fields["metric_buffer_limit"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if integer, ok := kv.Value.(*ast.Integer); ok {
				v, err := integer.Int()
				if err != nil {
					return nil, err
				}
				oc.MetricBufferLimit = int(v)
			}
		}
	}

	if node, ok := tbl.Fields["metric_batch_size"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if integer, ok := kv.Value.(*ast.Integer); ok {
				v, err := integer.Int()
				if err != nil {
					return nil, err
				}
				oc.MetricBatchSize = int(v)
			}
		}
	}

	if node, ok := tbl.Fields["alias"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				oc.Alias = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_override"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				oc.NameOverride = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_suffix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				oc.NameSuffix = str.Value
			}
		}
	}

	if node, ok := tbl.Fields["name_prefix"]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				oc.NamePrefix = str.Value
			}
		}
	}

	delete(tbl.Fields, "metric_buffer_limit")
	delete(tbl.Fields, "metric_batch_size")
	delete(tbl.Fields, "alias")
	delete(tbl.Fields, "name_override")
	delete(tbl.Fields, "name_suffix")
	delete(tbl.Fields, "name_prefix")

	return oc, nil
}

// unwrappable lets you retrieve the original telegraf.Processor from the
// StreamingProcessor. This is necessary because the toml Unmarshaller won't
// look inside composed types.
type unwrappable interface {
	Unwrap() telegraf.Processor
}

func getConfigDuration(tbl *ast.Table, key string, target *time.Duration) error {
	if node, ok := tbl.Fields[key]; ok {
		if kv, ok := node.(*ast.KeyValue); ok {
			if str, ok := kv.Value.(*ast.String); ok {
				d, err := time.ParseDuration(str.Value)
				if err != nil {
					return err
				}
				delete(tbl.Fields, key)
				*target = d
			}
		}
	}
	return nil
}

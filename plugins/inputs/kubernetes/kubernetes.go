package kubernetes

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shanas-swi/telegraf-v1.16.3"
	"github.com/shanas-swi/telegraf-v1.16.3/filter"
	"github.com/shanas-swi/telegraf-v1.16.3/internal"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/common/tls"
	"github.com/shanas-swi/telegraf-v1.16.3/plugins/inputs"
)

// Kubernetes represents the config object for the plugin
type Kubernetes struct {
	URL string

	// Bearer Token authorization file path
	BearerToken       string `toml:"bearer_token"`
	BearerTokenString string `toml:"bearer_token_string"`

	LabelInclude []string `toml:"label_include"`
	LabelExclude []string `toml:"label_exclude"`

	labelFilter filter.Filter

	// HTTP Timeout specified as a string - 3s, 1m, 1h
	ResponseTimeout internal.Duration

	tls.ClientConfig

	RoundTripper http.RoundTripper
}

var sampleConfig = `
  ## URL for the kubelet
  url = "http://127.0.0.1:10255"

  ## Use bearer token for authorization. ('bearer_token' takes priority)
  ## If both of these are empty, we'll use the default serviceaccount:
  ## at: /run/secrets/kubernetes.io/serviceaccount/token
  # bearer_token = "/path/to/bearer/token"
  ## OR
  # bearer_token_string = "abc_123"

  ## Pod labels to be added as tags.  An empty array for both include and
  ## exclude will include all labels.
  # label_include = []
  # label_exclude = ["*"]

  ## Set response_timeout (default 5 seconds)
  # response_timeout = "5s"

  ## Optional TLS Config
  # tls_ca = /path/to/cafile
  # tls_cert = /path/to/certfile
  # tls_key = /path/to/keyfile
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false
`

const (
	summaryEndpoint           = `%s/stats/summary`
	defaultServiceAccountPath = "/run/secrets/kubernetes.io/serviceaccount/token"
)

func init() {
	inputs.Add("kubernetes", func() telegraf.Input {
		return &Kubernetes{
			LabelInclude: []string{},
			LabelExclude: []string{"*"},
		}
	})
}

// SampleConfig returns a sample config
func (k *Kubernetes) SampleConfig() string {
	return sampleConfig
}

// Description returns the description of this plugin
func (k *Kubernetes) Description() string {
	return "Read metrics from the kubernetes kubelet api"
}

func (k *Kubernetes) Init() error {

	// If neither are provided, use the default service account.
	if k.BearerToken == "" && k.BearerTokenString == "" {
		k.BearerToken = defaultServiceAccountPath
	}

	if k.BearerToken != "" {
		token, err := ioutil.ReadFile(k.BearerToken)
		if err != nil {
			return err
		}
		k.BearerTokenString = strings.TrimSpace(string(token))
	}

	labelFilter, err := filter.NewIncludeExcludeFilter(k.LabelInclude, k.LabelExclude)
	if err != nil {
		return err
	}
	k.labelFilter = labelFilter

	return nil
}

// Gather collects kubernetes metrics from a given URL
func (k *Kubernetes) Gather(acc telegraf.Accumulator) error {
	acc.AddError(k.gatherSummary(k.URL, acc))
	return nil
}

func buildURL(endpoint string, base string) (*url.URL, error) {
	u := fmt.Sprintf(endpoint, base)
	addr, err := url.Parse(u)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse address '%s': %s", u, err)
	}
	return addr, nil
}

func (k *Kubernetes) gatherSummary(baseURL string, acc telegraf.Accumulator) error {
	summaryMetrics := &SummaryMetrics{}
	err := k.LoadJson(fmt.Sprintf("%s/stats/summary", baseURL), summaryMetrics)
	if err != nil {
		return err
	}

	podInfos, err := k.gatherPodInfo(baseURL)
	if err != nil {
		return err
	}
	buildSystemContainerMetrics(summaryMetrics, acc)
	buildNodeMetrics(summaryMetrics, acc)
	buildPodMetrics(baseURL, summaryMetrics, podInfos, k.labelFilter, acc)
	return nil
}

func buildSystemContainerMetrics(summaryMetrics *SummaryMetrics, acc telegraf.Accumulator) {
	for _, container := range summaryMetrics.Node.SystemContainers {
		tags := map[string]string{
			"node_name":      summaryMetrics.Node.NodeName,
			"container_name": container.Name,
		}
		fields := make(map[string]interface{})
		fields["cpu_usage_nanocores"] = container.CPU.UsageNanoCores
		fields["cpu_usage_core_nanoseconds"] = container.CPU.UsageCoreNanoSeconds
		fields["memory_usage_bytes"] = container.Memory.UsageBytes
		fields["memory_working_set_bytes"] = container.Memory.WorkingSetBytes
		fields["memory_rss_bytes"] = container.Memory.RSSBytes
		fields["memory_page_faults"] = container.Memory.PageFaults
		fields["memory_major_page_faults"] = container.Memory.MajorPageFaults
		fields["rootfs_available_bytes"] = container.RootFS.AvailableBytes
		fields["rootfs_capacity_bytes"] = container.RootFS.CapacityBytes
		fields["logsfs_available_bytes"] = container.LogsFS.AvailableBytes
		fields["logsfs_capacity_bytes"] = container.LogsFS.CapacityBytes
		acc.AddFields("kubernetes_system_container", fields, tags)
	}
}

func buildNodeMetrics(summaryMetrics *SummaryMetrics, acc telegraf.Accumulator) {
	tags := map[string]string{
		"node_name": summaryMetrics.Node.NodeName,
	}
	fields := make(map[string]interface{})
	fields["cpu_usage_nanocores"] = summaryMetrics.Node.CPU.UsageNanoCores
	fields["cpu_usage_core_nanoseconds"] = summaryMetrics.Node.CPU.UsageCoreNanoSeconds
	fields["memory_available_bytes"] = summaryMetrics.Node.Memory.AvailableBytes
	fields["memory_usage_bytes"] = summaryMetrics.Node.Memory.UsageBytes
	fields["memory_working_set_bytes"] = summaryMetrics.Node.Memory.WorkingSetBytes
	fields["memory_rss_bytes"] = summaryMetrics.Node.Memory.RSSBytes
	fields["memory_page_faults"] = summaryMetrics.Node.Memory.PageFaults
	fields["memory_major_page_faults"] = summaryMetrics.Node.Memory.MajorPageFaults
	fields["network_rx_bytes"] = summaryMetrics.Node.Network.RXBytes
	fields["network_rx_errors"] = summaryMetrics.Node.Network.RXErrors
	fields["network_tx_bytes"] = summaryMetrics.Node.Network.TXBytes
	fields["network_tx_errors"] = summaryMetrics.Node.Network.TXErrors
	fields["fs_available_bytes"] = summaryMetrics.Node.FileSystem.AvailableBytes
	fields["fs_capacity_bytes"] = summaryMetrics.Node.FileSystem.CapacityBytes
	fields["fs_used_bytes"] = summaryMetrics.Node.FileSystem.UsedBytes
	fields["runtime_image_fs_available_bytes"] = summaryMetrics.Node.Runtime.ImageFileSystem.AvailableBytes
	fields["runtime_image_fs_capacity_bytes"] = summaryMetrics.Node.Runtime.ImageFileSystem.CapacityBytes
	fields["runtime_image_fs_used_bytes"] = summaryMetrics.Node.Runtime.ImageFileSystem.UsedBytes
	acc.AddFields("kubernetes_node", fields, tags)
}

func (k *Kubernetes) gatherPodInfo(baseURL string) ([]Metadata, error) {
	var podApi Pods
	err := k.LoadJson(fmt.Sprintf("%s/pods", baseURL), &podApi)
	if err != nil {
		return nil, err
	}
	var podInfos []Metadata
	for _, podMetadata := range podApi.Items {
		podInfos = append(podInfos, podMetadata.Metadata)
	}
	return podInfos, nil
}

func (k *Kubernetes) LoadJson(url string, v interface{}) error {
	var req, err = http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	var resp *http.Response
	tlsCfg, err := k.ClientConfig.TLSConfig()
	if err != nil {
		return err
	}
	if k.RoundTripper == nil {
		if k.ResponseTimeout.Duration < time.Second {
			k.ResponseTimeout.Duration = time.Second * 5
		}
		k.RoundTripper = &http.Transport{
			TLSHandshakeTimeout:   5 * time.Second,
			TLSClientConfig:       tlsCfg,
			ResponseHeaderTimeout: k.ResponseTimeout.Duration,
		}
	}
	req.Header.Set("Authorization", "Bearer "+k.BearerTokenString)
	req.Header.Add("Accept", "application/json")
	resp, err = k.RoundTripper.RoundTrip(req)
	if err != nil {
		return fmt.Errorf("error making HTTP request to %s: %s", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP status %s", url, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(v)
	if err != nil {
		return fmt.Errorf(`Error parsing response: %s`, err)
	}

	return nil
}

func buildPodMetrics(baseURL string, summaryMetrics *SummaryMetrics, podInfo []Metadata, labelFilter filter.Filter, acc telegraf.Accumulator) {
	for _, pod := range summaryMetrics.Pods {
		for _, container := range pod.Containers {
			tags := map[string]string{
				"node_name":      summaryMetrics.Node.NodeName,
				"namespace":      pod.PodRef.Namespace,
				"container_name": container.Name,
				"pod_name":       pod.PodRef.Name,
			}
			for _, info := range podInfo {
				if info.Name == pod.PodRef.Name && info.Namespace == pod.PodRef.Namespace {
					for k, v := range info.Labels {
						if labelFilter.Match(k) {
							tags[k] = v
						}
					}
				}
			}

			fields := make(map[string]interface{})
			fields["cpu_usage_nanocores"] = container.CPU.UsageNanoCores
			fields["cpu_usage_core_nanoseconds"] = container.CPU.UsageCoreNanoSeconds
			fields["memory_usage_bytes"] = container.Memory.UsageBytes
			fields["memory_working_set_bytes"] = container.Memory.WorkingSetBytes
			fields["memory_rss_bytes"] = container.Memory.RSSBytes
			fields["memory_page_faults"] = container.Memory.PageFaults
			fields["memory_major_page_faults"] = container.Memory.MajorPageFaults
			fields["rootfs_available_bytes"] = container.RootFS.AvailableBytes
			fields["rootfs_capacity_bytes"] = container.RootFS.CapacityBytes
			fields["rootfs_used_bytes"] = container.RootFS.UsedBytes
			fields["logsfs_available_bytes"] = container.LogsFS.AvailableBytes
			fields["logsfs_capacity_bytes"] = container.LogsFS.CapacityBytes
			fields["logsfs_used_bytes"] = container.LogsFS.UsedBytes
			acc.AddFields("kubernetes_pod_container", fields, tags)
		}

		for _, volume := range pod.Volumes {
			tags := map[string]string{
				"node_name":   summaryMetrics.Node.NodeName,
				"pod_name":    pod.PodRef.Name,
				"namespace":   pod.PodRef.Namespace,
				"volume_name": volume.Name,
			}
			fields := make(map[string]interface{})
			fields["available_bytes"] = volume.AvailableBytes
			fields["capacity_bytes"] = volume.CapacityBytes
			fields["used_bytes"] = volume.UsedBytes
			acc.AddFields("kubernetes_pod_volume", fields, tags)
		}

		tags := map[string]string{
			"node_name": summaryMetrics.Node.NodeName,
			"pod_name":  pod.PodRef.Name,
			"namespace": pod.PodRef.Namespace,
		}
		fields := make(map[string]interface{})
		fields["rx_bytes"] = pod.Network.RXBytes
		fields["rx_errors"] = pod.Network.RXErrors
		fields["tx_bytes"] = pod.Network.TXBytes
		fields["tx_errors"] = pod.Network.TXErrors
		acc.AddFields("kubernetes_pod_network", fields, tags)
	}
}

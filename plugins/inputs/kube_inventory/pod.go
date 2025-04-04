package kube_inventory

import (
	"context"

	v1 "github.com/ericchiang/k8s/apis/core/v1"

	"github.com/shanas-swi/telegraf-v1.16.3"
)

func collectPods(ctx context.Context, acc telegraf.Accumulator, ki *KubernetesInventory) {
	list, err := ki.client.getPods(ctx)
	if err != nil {
		acc.AddError(err)
		return
	}
	for _, p := range list.Items {
		if err = ki.gatherPod(*p, acc); err != nil {
			acc.AddError(err)
			return
		}
	}
}

func (ki *KubernetesInventory) gatherPod(p v1.Pod, acc telegraf.Accumulator) error {
	if p.Metadata.CreationTimestamp.GetSeconds() == 0 && p.Metadata.CreationTimestamp.GetNanos() == 0 {
		return nil
	}

	for i, cs := range p.Status.ContainerStatuses {
		c := p.Spec.Containers[i]
		gatherPodContainer(*p.Spec.NodeName, ki, p, *cs, *c, acc)
	}

	return nil
}

func gatherPodContainer(nodeName string, ki *KubernetesInventory, p v1.Pod, cs v1.ContainerStatus, c v1.Container, acc telegraf.Accumulator) {
	stateCode := 3
	stateReason := ""
	state := "unknown"

	switch {
	case cs.State.Running != nil:
		stateCode = 0
		state = "running"
	case cs.State.Terminated != nil:
		stateCode = 1
		state = "terminated"
		stateReason = cs.State.Terminated.GetReason()
	case cs.State.Waiting != nil:
		stateCode = 2
		state = "waiting"
		stateReason = cs.State.Waiting.GetReason()
	}

	readiness := "unready"
	if cs.GetReady() {
		readiness = "ready"
	}

	fields := map[string]interface{}{
		"restarts_total":    cs.GetRestartCount(),
		"state_code":        stateCode,
		"terminated_reason": cs.State.Terminated.GetReason(),
	}

	if stateReason != "" {
		fields["state_reason"] = stateReason
	}

	tags := map[string]string{
		"container_name": *c.Name,
		"namespace":      *p.Metadata.Namespace,
		"node_name":      *p.Spec.NodeName,
		"pod_name":       *p.Metadata.Name,
		"state":          state,
		"readiness":      readiness,
	}
	for key, val := range p.GetSpec().GetNodeSelector() {
		if ki.selectorFilter.Match(key) {
			tags["node_selector_"+key] = val
		}
	}

	req := c.Resources.Requests
	lim := c.Resources.Limits

	for resourceName, val := range req {
		switch resourceName {
		case "cpu":
			fields["resource_requests_millicpu_units"] = convertQuantity(val.GetString_(), 1000)
		case "memory":
			fields["resource_requests_memory_bytes"] = convertQuantity(val.GetString_(), 1)
		}
	}
	for resourceName, val := range lim {
		switch resourceName {
		case "cpu":
			fields["resource_limits_millicpu_units"] = convertQuantity(val.GetString_(), 1000)
		case "memory":
			fields["resource_limits_memory_bytes"] = convertQuantity(val.GetString_(), 1)
		}
	}

	acc.AddFields(podContainerMeasurement, fields, tags)
}

package kube_inventory

import (
	"context"
	"time"

	v1 "github.com/ericchiang/k8s/apis/apps/v1"
	"github.com/shanas-swi/telegraf-v1.16.3"
)

func collectDeployments(ctx context.Context, acc telegraf.Accumulator, ki *KubernetesInventory) {
	list, err := ki.client.getDeployments(ctx)
	if err != nil {
		acc.AddError(err)
		return
	}
	for _, d := range list.Items {
		if err = ki.gatherDeployment(*d, acc); err != nil {
			acc.AddError(err)
			return
		}
	}
}

func (ki *KubernetesInventory) gatherDeployment(d v1.Deployment, acc telegraf.Accumulator) error {
	fields := map[string]interface{}{
		"replicas_available":   d.Status.GetAvailableReplicas(),
		"replicas_unavailable": d.Status.GetUnavailableReplicas(),
		"created":              time.Unix(d.Metadata.CreationTimestamp.GetSeconds(), int64(d.Metadata.CreationTimestamp.GetNanos())).UnixNano(),
	}
	tags := map[string]string{
		"deployment_name": d.Metadata.GetName(),
		"namespace":       d.Metadata.GetNamespace(),
	}
	for key, val := range d.GetSpec().GetSelector().GetMatchLabels() {
		if ki.selectorFilter.Match(key) {
			tags["selector_"+key] = val
		}
	}

	acc.AddFields(deploymentMeasurement, fields, tags)

	return nil
}

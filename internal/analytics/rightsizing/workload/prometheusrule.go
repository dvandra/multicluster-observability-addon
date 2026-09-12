package workload

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GeneratePrometheusRule builds a PrometheusRule containing workload and pod
// level recording rules, including CPU/memory request, limit, usage, and
// recommendation series.
func GeneratePrometheusRule(configData rightsizing.RSConfigMapData) (monitoringv1.PrometheusRule, error) {
	nsFilter, err := rightsizing.BuildNamespaceFilter(configData.PrometheusRuleConfig)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	labelJoin, err := rightsizing.BuildLabelJoin(configData.PrometheusRuleConfig.LabelFilterCriteria)
	if err != nil {
		return monitoringv1.PrometheusRule{}, err
	}

	rb := rightsizing.NewRuleBuilder(labelJoin)

	return monitoringv1.PrometheusRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rightsizing.WorkloadPrometheusRuleName,
			Namespace: rightsizing.MonitoringNamespace,
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "PrometheusRule",
			APIVersion: "monitoring.coreos.com/v1",
		},
		Spec: monitoringv1.PrometheusRuleSpec{
			Groups: []monitoringv1.RuleGroup{
				{
					Name:     "acm-right-sizing-workload-5m.rules",
					Interval: &rightsizing.Duration5m,
					Rules:    buildWorkloadRules5m(nsFilter, rb),
				},
				{
					Name:     "acm-right-sizing-workload-1d.rules",
					Interval: &rightsizing.Duration1d,
					Rules:    buildWorkloadRules1d(configData, rb),
				},
			},
		},
	}, nil
}

// podWorkloadRelabelExpr maps pods to their owning workloads, handling
// Deployments (via ReplicaSets), StatefulSets, DaemonSets, CronJobs (via Jobs),
// standalone Jobs, and standalone ReplicaSets.
func podWorkloadRelabelExpr(nsFilter string) string {
	return fmt.Sprintf(
		`(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        kube_pod_owner{%s, owner_kind=~"StatefulSet|DaemonSet"},
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "$1", "owner_kind", "(.*)"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          label_replace(
		            kube_pod_owner{%s, owner_kind="ReplicaSet"},
		            "replicaset", "$1", "owner_name", "(.*)"
		          )
		          * on (namespace, replicaset) group_left(owner_name)
		            topk by (namespace, replicaset) (
		              1,
		              max by (namespace, replicaset, owner_name) (
		                kube_replicaset_owner{%s, owner_kind="Deployment"}
		              )
		            )
		        ),
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "Deployment", "workload", ".*"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          label_replace(
		            kube_pod_owner{%s, owner_kind="ReplicaSet"},
		            "replicaset", "$1", "owner_name", "(.*)"
		          )
		          unless on (namespace, replicaset)
		            kube_replicaset_owner{%s, owner_kind="Deployment"}
		        ),
		        "workload", "$1", "replicaset", "(.*)"
		      ),
		      "workload_type", "ReplicaSet", "workload", ".*"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          label_replace(
		            kube_pod_owner{%s, owner_kind="Job"},
		            "job_name", "$1", "owner_name", "(.*)"
		          )
		          * on (namespace, job_name) group_left(owner_name)
		            max by (namespace, job_name, owner_name) (
		              kube_job_owner{%s, owner_kind="CronJob"}
		            )
		        ),
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "CronJob", "workload", ".*"
		    )
		  )
		)
		or
		(
		  max by (namespace, pod, workload, workload_type) (
		    label_replace(
		      label_replace(
		        (
		          kube_pod_owner{%s, owner_kind="Job"}
		          unless on (namespace, owner_name)
		            max by (namespace, owner_name) (
		              label_replace(
		                kube_job_owner{%s, owner_kind="CronJob"},
		                "owner_name", "$1", "job_name", "(.*)"
		              )
		            )
		        ),
		        "workload", "$1", "owner_name", "(.*)"
		      ),
		      "workload_type", "Job", "workload", ".*"
		    )
		  )
		)`,
		nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter, nsFilter,
	)
}

func resource5mExpr(metric, resource, nsFilter, byLabels string) string {
	return fmt.Sprintf(
		`max_over_time(sum by (%s) (
		  %s{%s, resource="%s", container!=""}
		  * on (namespace, pod) group_left(workload, workload_type)
		    acm_rs:pod_workload:relabel:5m
		)[5m:])`, byLabels, metric, nsFilter, resource)
}

func usage5mExpr(metric, nsFilter, byLabels string) string {
	return fmt.Sprintf(
		`max_over_time(sum by (%s) (
		  %s{%s, container!=""}
		  * on (namespace, pod) group_left(workload, workload_type)
		    acm_rs:pod_workload:relabel:5m
		)[5m:])`, byLabels, metric, nsFilter)
}

func buildWorkloadRules5m(nsFilter string, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	podBy := "namespace, pod, workload, workload_type"
	wlBy := "namespace, workload, workload_type"

	return []monitoringv1.Rule{
		rb.Rule("acm_rs:pod_workload:relabel:5m", podWorkloadRelabelExpr(nsFilter)),

		rb.Rule("acm_rs:pod:cpu_request:5m", resource5mExpr("kube_pod_container_resource_requests", "cpu", nsFilter, podBy)),
		rb.Rule("acm_rs:pod:cpu_limit:5m", resource5mExpr("kube_pod_container_resource_limits", "cpu", nsFilter, podBy)),
		rb.Rule("acm_rs:pod:cpu_usage:5m", usage5mExpr("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate", nsFilter, podBy)),
		rb.Rule("acm_rs:pod:memory_request:5m", resource5mExpr("kube_pod_container_resource_requests", "memory", nsFilter, podBy)),
		rb.Rule("acm_rs:pod:memory_limit:5m", resource5mExpr("kube_pod_container_resource_limits", "memory", nsFilter, podBy)),
		rb.Rule("acm_rs:pod:memory_usage:5m", usage5mExpr("container_memory_working_set_bytes", nsFilter, podBy)),

		rb.Rule("acm_rs:workload:cpu_request:5m", resource5mExpr("kube_pod_container_resource_requests", "cpu", nsFilter, wlBy)),
		rb.Rule("acm_rs:workload:cpu_limit:5m", resource5mExpr("kube_pod_container_resource_limits", "cpu", nsFilter, wlBy)),
		rb.Rule("acm_rs:workload:cpu_usage:5m", usage5mExpr("node_namespace_pod_container:container_cpu_usage_seconds_total:sum_irate", nsFilter, wlBy)),
		rb.Rule("acm_rs:workload:memory_request:5m", resource5mExpr("kube_pod_container_resource_requests", "memory", nsFilter, wlBy)),
		rb.Rule("acm_rs:workload:memory_limit:5m", resource5mExpr("kube_pod_container_resource_limits", "memory", nsFilter, wlBy)),
		rb.Rule("acm_rs:workload:memory_usage:5m", usage5mExpr("container_memory_working_set_bytes", nsFilter, wlBy)),
	}
}

func buildWorkloadRules1d(configData rightsizing.RSConfigMapData, rb *rightsizing.RuleBuilder) []monitoringv1.Rule {
	rp := configData.PrometheusRuleConfig.RecommendationPercentage
	if rp == 0 {
		rp = rightsizing.DefaultRecommendationPercentage
	}

	cpuProfiles := rightsizing.ResolveCpuProfiles(configData.PrometheusRuleConfig)
	memProfiles := rightsizing.ResolveMemoryProfiles(configData.PrometheusRuleConfig)

	var rules []monitoringv1.Rule
	for _, profile := range cpuProfiles {
		prb := rb.WithProfile(profile.Name)
		rules = append(rules,
			prb.RuleWithLabels("acm_rs:pod:cpu_request", profile.AggExpr("acm_rs:pod:cpu_request:5m")),
			prb.RuleWithLabels("acm_rs:pod:cpu_limit", profile.AggExpr("acm_rs:pod:cpu_limit:5m")),
			prb.RuleWithLabels("acm_rs:pod:cpu_usage", profile.AggExpr("acm_rs:pod:cpu_usage:5m")),
			prb.RuleWithLabels("acm_rs:pod:cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:pod:cpu_usage:5m", rp, profile)),
			prb.RuleWithLabels("acm_rs:workload:cpu_request", profile.AggExpr("acm_rs:workload:cpu_request:5m")),
			prb.RuleWithLabels("acm_rs:workload:cpu_limit", profile.AggExpr("acm_rs:workload:cpu_limit:5m")),
			prb.RuleWithLabels("acm_rs:workload:cpu_usage", profile.AggExpr("acm_rs:workload:cpu_usage:5m")),
			prb.RuleWithLabels("acm_rs:workload:cpu_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:workload:cpu_usage:5m", rp, profile)),
		)
	}
	for _, profile := range memProfiles {
		prb := rb.WithProfile(profile.Name)
		rules = append(rules,
			prb.RuleWithLabels("acm_rs:pod:memory_request", profile.AggExpr("acm_rs:pod:memory_request:5m")),
			prb.RuleWithLabels("acm_rs:pod:memory_limit", profile.AggExpr("acm_rs:pod:memory_limit:5m")),
			prb.RuleWithLabels("acm_rs:pod:memory_usage", profile.AggExpr("acm_rs:pod:memory_usage:5m")),
			prb.RuleWithLabels("acm_rs:pod:memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:pod:memory_usage:5m", rp, profile)),
			prb.RuleWithLabels("acm_rs:workload:memory_request", profile.AggExpr("acm_rs:workload:memory_request:5m")),
			prb.RuleWithLabels("acm_rs:workload:memory_limit", profile.AggExpr("acm_rs:workload:memory_limit:5m")),
			prb.RuleWithLabels("acm_rs:workload:memory_usage", profile.AggExpr("acm_rs:workload:memory_usage:5m")),
			prb.RuleWithLabels("acm_rs:workload:memory_recommendation", rightsizing.BuildProfiledRecommendationExpr("acm_rs:workload:memory_usage:5m", rp, profile)),
		)
	}
	return rules
}

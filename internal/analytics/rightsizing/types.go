package rightsizing

// RS resource labels — must match MCO's rsutility.RSLabels() so that MCO can
// discover and manage these resources during mode switches and cleanup.
// In the future when only MCOA mode is supported, these can use MCOA-specific labels.
const (
	RSManagedByLabel = "observability.open-cluster-management.io/managed-by"
	RSManagedByValue = "analytics-rightsizing"
)

// RSLabels returns the standard labels applied to all right-sizing hub resources.
func RSLabels() map[string]string {
	return map[string]string{RSManagedByLabel: RSManagedByValue}
}

// Common constants
const (
	DefaultRecommendationPercentage = 110
	MonitoringNamespace             = "openshift-monitoring"

	// Namespace right-sizing constants
	NamespacePrometheusRuleName = "acm-rs-namespace-prometheus-rules"
	NamespaceConfigMapName      = "rs-namespace-config"

	// Virtualization right-sizing constants
	VirtualizationPrometheusRuleName = "acm-rs-virt-prometheus-rules"
	VirtualizationConfigMapName      = "rs-virt-config"
)

// RSLabelFilter represents label filtering criteria for right-sizing
type RSLabelFilter struct {
	LabelName         string   `yaml:"labelName" json:"labelName"`
	InclusionCriteria []string `yaml:"inclusionCriteria,omitempty" json:"inclusionCriteria,omitempty"`
	ExclusionCriteria []string `yaml:"exclusionCriteria,omitempty" json:"exclusionCriteria,omitempty"`
}

// RSPrometheusRuleConfig represents the Prometheus rule configuration for right-sizing
type RSPrometheusRuleConfig struct {
	NamespaceFilterCriteria struct {
		InclusionCriteria []string `yaml:"inclusionCriteria" json:"inclusionCriteria"`
		ExclusionCriteria []string `yaml:"exclusionCriteria" json:"exclusionCriteria"`
	} `yaml:"namespaceFilterCriteria" json:"namespaceFilterCriteria"`
	LabelFilterCriteria      []RSLabelFilter `yaml:"labelFilterCriteria" json:"labelFilterCriteria"`
	RecommendationPercentage int             `yaml:"recommendationPercentage" json:"recommendationPercentage"`
}

// RSConfigMapData represents the configmap data structure for right-sizing
type RSConfigMapData struct {
	PrometheusRuleConfig RSPrometheusRuleConfig `yaml:"prometheusRuleConfig" json:"prometheusRuleConfig"`
}

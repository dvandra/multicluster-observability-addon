package main

import (
	"encoding/json"
	"fmt"
	"os"
	rsperses "github.com/stolostron/multicluster-observability-addon/internal/perses/dashboards/rightsizing"
)

func main() {
	db, err := rsperses.BuildVMOverview("observability-analytics", "rbac-query-proxy-datasource", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	spec, _ := json.Marshal(db.Dashboard.Spec)
	fmt.Printf("apiVersion: perses.dev/v1alpha1\nkind: PersesDashboard\nmetadata:\n  name: %s\n  namespace: observability-analytics\nspec: %s\n", db.Dashboard.Metadata.Name, string(spec))
}

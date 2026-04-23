package handlers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stolostron/multicluster-observability-addon/internal/addon"
	addoncfg "github.com/stolostron/multicluster-observability-addon/internal/addon/config"
	"github.com/stolostron/multicluster-observability-addon/internal/analytics/rightsizing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func setupTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, corev1.AddToScheme(scheme))
	return scheme
}

func newTestOptionsBuilder(t *testing.T, objs ...runtime.Object) *OptionsBuilder {
	t.Helper()
	scheme := setupTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()
	return &OptionsBuilder{
		Client: c,
		Logger: logr.Discard(),
	}
}

func newPlatformOpts(nsEnabled, virtEnabled bool) addon.Options {
	return addon.Options{
		Platform: addon.PlatformOptions{
			Enabled: true,
			AnalyticsOptions: addon.AnalyticsOptions{
				RightSizing: addon.RightSizingOptions{
					NamespaceEnabled:      nsEnabled,
					VirtualizationEnabled: virtEnabled,
				},
			},
		},
	}
}

func createTestConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: addoncfg.InstallNamespace,
		},
		Data: map[string]string{"prometheusRuleConfig": "test"},
	}
}

func TestRSConfigMapPredicate(t *testing.T) {
	pred := RSConfigMapPredicate()

	rsNsCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}}
	rsVirtCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}}
	unrelatedCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "other-config", Namespace: addoncfg.InstallNamespace,
	}}
	wrongNsCM := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: rightsizing.NamespaceConfigMapName, Namespace: "other-namespace",
	}}

	assert.True(t, pred.CreateFunc(event.CreateEvent{Object: rsNsCM}))
	assert.True(t, pred.CreateFunc(event.CreateEvent{Object: rsVirtCM}))
	assert.False(t, pred.CreateFunc(event.CreateEvent{Object: unrelatedCM}))
	assert.False(t, pred.CreateFunc(event.CreateEvent{Object: wrongNsCM}))

	assert.True(t, pred.UpdateFunc(event.UpdateEvent{ObjectNew: rsNsCM}))
	assert.False(t, pred.UpdateFunc(event.UpdateEvent{ObjectNew: unrelatedCM}))

	assert.False(t, pred.DeleteFunc(event.DeleteEvent{Object: rsNsCM}))
	assert.False(t, pred.DeleteFunc(event.DeleteEvent{Object: unrelatedCM}))

	assert.False(t, pred.GenericFunc(event.GenericEvent{Object: rsNsCM}))
}

func TestReconcileRSResources_CleanupNamespace(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := newPlatformOpts(false, true)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "namespace configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.NoError(t, err, "virtualization configmap should still exist")
}

func TestReconcileRSResources_CleanupVirtualization(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := newPlatformOpts(true, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.NoError(t, err, "namespace configmap should still exist")
}

func TestReconcileRSResources_CleanupBoth(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)
	virtCM := createTestConfigMap(rightsizing.VirtualizationConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM, virtCM)
	ctx := context.TODO()

	opts := newPlatformOpts(false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "namespace configmap should be deleted")

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.VirtualizationConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "virtualization configmap should be deleted")
}

func TestReconcileRSResources_CleanupIdempotent(t *testing.T) {
	ob := newTestOptionsBuilder(t)
	ctx := context.TODO()

	opts := newPlatformOpts(false, false)
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)
}

func TestReconcileRSResources_PlatformDisabledCleansUp(t *testing.T) {
	nsCM := createTestConfigMap(rightsizing.NamespaceConfigMapName)

	ob := newTestOptionsBuilder(t, nsCM)
	ctx := context.TODO()

	opts := addon.Options{
		Platform: addon.PlatformOptions{Enabled: false},
	}
	err := ob.ReconcileRSResources(ctx, opts)
	require.NoError(t, err)

	err = ob.Client.Get(ctx, types.NamespacedName{
		Name: rightsizing.NamespaceConfigMapName, Namespace: addoncfg.InstallNamespace,
	}, &corev1.ConfigMap{})
	assert.True(t, apierrors.IsNotFound(err), "configmap should be deleted when both RS features are disabled")
}

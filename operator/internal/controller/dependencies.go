package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
	"github.com/dasmlab/glooscap-operator/pkg/outline"
)

// OutlineClientFactory constructs Outline clients for WikiTargets.
type OutlineClientFactory interface {
	New(ctx context.Context, c client.Client, target *wikiv1alpha1.WikiTarget) (*outline.Client, error)
}

// DefaultOutlineClientFactory reads secrets from Kubernetes and instantiates clients.
type DefaultOutlineClientFactory struct{}

// New creates an Outline client using the service account secret referenced by the target.
func (DefaultOutlineClientFactory) New(ctx context.Context, c client.Client, target *wikiv1alpha1.WikiTarget) (*outline.Client, error) {
	if target.Spec.ServiceAccountSecretRef.Name == "" {
		return nil, fmt.Errorf("outline factory: service account secret ref is empty")
	}

	var secret corev1.Secret
	key := types.NamespacedName{
		Namespace: target.Namespace,
		Name:      target.Spec.ServiceAccountSecretRef.Name,
	}
	if err := c.Get(ctx, key, &secret); err != nil {
		return nil, fmt.Errorf("outline factory: get secret %s: %w", key, err)
	}

	keyName := target.Spec.ServiceAccountSecretRef.Key
	if keyName == "" {
		keyName = "token"
	}

	tokenBytes, ok := secret.Data[keyName]
	if !ok {
		return nil, fmt.Errorf("outline factory: key %q not found in secret %s", keyName, key)
	}

	client, err := outline.NewClient(outline.Config{
		BaseURL: target.Spec.URI,
		Token:   string(tokenBytes),
	})
	if err != nil {
		return nil, fmt.Errorf("outline factory: %w", err)
	}
	return client, nil
}


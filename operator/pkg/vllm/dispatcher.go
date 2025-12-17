package vllm

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Mode represents the backend execution strategy.
type Mode string

const (
	ModeTektonJob Mode = "TektonJob"
	ModeInline    Mode = "InlineLLM"
)

// Dispatcher handles sending inference requests.
type Dispatcher interface {
	Dispatch(ctx context.Context, req Request) error
}

// Request models a translation dispatch.
type Request struct {
	JobName      string
	Namespace    string
	PageID       string
	LanguageTag  string
	SourceTarget string
	Mode         Mode
}

// TektonJobDispatcher submits Kubernetes Jobs that in turn invoke the vLLM API.
type TektonJobDispatcher struct {
	Client       client.Client
	Namespace    string
	Image        string
	APIServerURL string
}

// Dispatch creates or patches a Job that runs the translation-runner container.
// The runner reads the TranslationJob CR and processes the translation.
func (d *TektonJobDispatcher) Dispatch(ctx context.Context, req Request) error {
	if d.Client == nil {
		return fmt.Errorf("translation dispatcher: client is nil")
	}
	ns := req.Namespace
	if ns == "" {
		ns = d.Namespace
	}
	name := fmt.Sprintf("translation-%s", req.JobName)

	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch/v1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "glooscap-operator",
				"glooscap.dasmlab.org/job":     req.JobName,
			},
		},
		Spec: batchv1.JobSpec{
			// Set TTL to automatically clean up completed/failed jobs after 1 hour
			// This prevents accumulation of failed job pods
			TTLSecondsAfterFinished: ptr.To(int32(3600)), // 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "glooscap-operator",
						"glooscap.dasmlab.org/job":     req.JobName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "dasmlab-ghcr-pull"}, // Use same secret as operator
					},
					Containers: []corev1.Container{
						{
							Name:            "translation-runner",
							Image:           d.Image,
							// Use IfNotPresent to allow operation in isolated environments (e.g., VPN-connected)
							// where GHCR may be unreachable. Once the image is pulled, it will be cached
							// and reused. For fresh pulls, ensure the image is available before isolation.
							ImagePullPolicy: corev1.PullIfNotPresent,
							Args: []string{
								"--translation-job", fmt.Sprintf("%s/%s", ns, req.JobName),
							},
							Env: []corev1.EnvVar{
								{
									Name: "TRANSLATION_SERVICE_ADDR",
									ValueFrom: &corev1.EnvVarSource{
										ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "glooscap-config",
											},
											Key:      "translation-service-addr",
											Optional: ptr.To(true),
										},
									},
								},
							},
						},
					},
					ServiceAccountName: "operator-controller-manager", // Use operator's service account which has RBAC
				},
			},
		},
	}

	return d.Client.Patch(ctx, job, client.Apply, &client.PatchOptions{
		Force:        ptr.To(true),
		FieldManager: "glooscap-operator",
	})
}

// InlineDispatcher is a placeholder that will call the vLLM API directly in-process.
type InlineDispatcher struct {
	Do func(ctx context.Context, req Request) error
}

// Dispatch executes the inline function or returns an error if none is defined.
func (d *InlineDispatcher) Dispatch(ctx context.Context, req Request) error {
	if d.Do == nil {
		return fmt.Errorf("inline dispatcher not configured")
	}
	return d.Do(ctx, req)
}

// ModeFromString converts a string to Mode with fallback.
func ModeFromString(val string) Mode {
	switch Mode(val) {
	case ModeInline:
		return ModeInline
	default:
		return ModeTektonJob
	}
}

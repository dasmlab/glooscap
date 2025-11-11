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

// Dispatch creates or patches a Job that invokes the vLLM gateway.
func (d *TektonJobDispatcher) Dispatch(ctx context.Context, req Request) error {
	if d.Client == nil {
		return fmt.Errorf("vllm dispatcher: client is nil")
	}
	ns := req.Namespace
	if ns == "" {
		ns = d.Namespace
	}
	name := fmt.Sprintf("vllm-%s", req.JobName)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "inference",
							Image: d.Image,
							Args: []string{
								"--job-id", req.JobName,
								"--page-id", req.PageID,
								"--target", req.SourceTarget,
								"--language", req.LanguageTag,
								"--vllm-url", d.APIServerURL,
							},
						},
					},
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

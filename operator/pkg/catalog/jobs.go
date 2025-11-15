package catalog

import (
	"sync"

	wikiv1alpha1 "github.com/dasmlab/glooscap-operator/api/v1alpha1"
)

// JobStore keeps translation job statuses for UI consumption.
type JobStore struct {
	mu   sync.RWMutex
	jobs map[string]Job
}

// NewJobStore returns a new JobStore.
func NewJobStore() *JobStore {
	return &JobStore{
		jobs: make(map[string]Job),
	}
}

// Job aggregates spec metadata with status for UI consumption.
type Job struct {
	Status    wikiv1alpha1.TranslationJobStatus `json:"status"`
	Pipeline  string                            `json:"pipeline"`
	TargetRef string                            `json:"targetRef"`
	PageID    string                            `json:"pageId"`
	PageTitle string                            `json:"pageTitle"`
}

// Update records the latest status for the job.
func (s *JobStore) Update(job *wikiv1alpha1.TranslationJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := job.Status.DeepCopy()
	s.jobs[job.Name] = Job{
		Status:    *status,
		Pipeline:  string(job.Spec.Pipeline),
		TargetRef: job.Spec.Source.TargetRef,
		PageID:    job.Spec.Source.PageID,
		PageTitle: job.Spec.Parameters["pageTitle"],
	}
}

// List returns all job statuses.
func (s *JobStore) List() map[string]Job {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Job, len(s.jobs))
	for k, v := range s.jobs {
		out[k] = v
	}
	return out
}

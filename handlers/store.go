package handlers

// store.go is a small, mutex-guarded in-memory store for graded records.
// The roster is capped so a runaway client cannot exhaust server memory.

import (
	"sync"

	"github.com/israeloluwasegun293-stars/classedge/grading"
)

type resultStore struct {
	mu      sync.Mutex
	records []grading.StudentRecord
	metrics grading.ClassMetrics
}

func newResultStore() *resultStore {
	return &resultStore{metrics: grading.NewClassMetrics()}
}

func (s *resultStore) add(rec grading.StudentRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
	s.metrics.Add(rec)
	s.metrics.Finalize()

	// Keep only the most recent MaxStudents records.
	if len(s.records) > grading.MaxStudents {
		s.records = s.records[len(s.records)-grading.MaxStudents:]
	}
}

func (s *resultStore) snapshot() ([]grading.StudentRecord, grading.ClassMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]grading.StudentRecord, len(s.records))
	copy(out, s.records)
	return out, s.metrics
}

func (s *resultStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = nil
	s.metrics = grading.NewClassMetrics()
}

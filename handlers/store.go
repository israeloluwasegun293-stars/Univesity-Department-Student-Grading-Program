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
	s.addBatch([]grading.StudentRecord{rec})
}

// addBatch commits a complete batch while holding the lock so readers never
// observe a partially stored batch.
func (s *resultStore) addBatch(records []grading.StudentRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, records...)

	// Keep only the most recent MaxStudents records.
	if len(s.records) > grading.MaxStudents {
		s.records = s.records[len(s.records)-grading.MaxStudents:]
	}
	s.rebuildMetrics()
}

func (s *resultStore) rebuildMetrics() {
	s.metrics = grading.NewClassMetrics()
	for _, rec := range s.records {
		s.metrics.Add(rec)
	}
	s.metrics.Finalize()
}

func (s *resultStore) snapshot() ([]grading.StudentRecord, grading.ClassMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]grading.StudentRecord, len(s.records))
	copy(out, s.records)
	metrics := s.metrics
	metrics.GradeCounts = make(map[string]int, len(s.metrics.GradeCounts))
	for grade, count := range s.metrics.GradeCounts {
		metrics.GradeCounts[grade] = count
	}
	return out, metrics
}

func (s *resultStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = nil
	s.metrics = grading.NewClassMetrics()
}

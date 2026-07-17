package testrun

import "sync"

type SuiteStore struct {
	m sync.Map
}

func NewSuiteStore() *SuiteStore {
	return &SuiteStore{}
}

func (s *SuiteStore) Get(id string) (*Suite, bool) {
	val, found := s.m.Load(id)
	if !found {
		return nil, false
	}
	return val.(*Suite), true
}

func (s *SuiteStore) Set(id string, suite *Suite) {
	s.m.Store(id, suite)
}

func (s *SuiteStore) Delete(id string) {
	s.m.Delete(id)
}

package main

import (
	"errors"
	"sort"
	"sync"
)

const maxKeyLength = 200

var (
	ErrEmptyKey     = errors.New("key must not be empty")
	ErrKeyTooLong   = errors.New("key must be at most 200 characters")
	ErrRolloutRange = errors.New("rollout_percent must be between 0 and 100")
	ErrFlagExists   = errors.New("flag already exists")
)

type Store struct {
	mu    sync.Mutex
	flags map[string]Flag
}

func NewStore() *Store {
	return &Store{flags: make(map[string]Flag)}
}

func (s *Store) Create(f Flag) error {
	if err := validateKey(f.Key); err != nil {
		return err
	}
	if f.RolloutPercent < 0 || f.RolloutPercent > 100 {
		return ErrRolloutRange
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.flags[f.Key]; exists {
		return ErrFlagExists
	}
	s.flags[f.Key] = f
	return nil
}

func (s *Store) List() []Flag {
	s.mu.Lock()
	defer s.mu.Unlock()

	flags := make([]Flag, 0, len(s.flags))
	for _, f := range s.flags {
		flags = append(flags, f)
	}
	sort.Slice(flags, func(i, j int) bool {
		return flags[i].Key < flags[j].Key
	})
	return flags
}

func (s *Store) Get(key string) (Flag, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, ok := s.flags[key]
	return f, ok
}

func (s *Store) Update(key string, f Flag) (Flag, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, ok := s.flags[key]
	if !ok {
		return Flag{}, false
	}

	if f.RolloutPercent < 0 || f.RolloutPercent > 100 {
		return Flag{}, false
	}

	existing.Enabled = f.Enabled
	existing.Description = f.Description
	existing.RolloutPercent = f.RolloutPercent
	s.flags[key] = existing
	return existing, true
}

func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.flags[key]; !ok {
		return false
	}
	delete(s.flags, key)
	return true
}

func validateKey(key string) error {
	if key == "" {
		return ErrEmptyKey
	}
	if len(key) > maxKeyLength {
		return ErrKeyTooLong
	}
	return nil
}

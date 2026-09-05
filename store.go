package main

import (
	"errors"
	"sync"
)

type Store struct {
	mu    sync.Mutex
	flags map[string]Flag
}

func NewStore() *Store {
	return &Store{flags: make(map[string]Flag)}
}

func (s *Store) Create(f Flag) error {
	return errors.New("not implemented")
}

func (s *Store) List() []Flag {
	return nil
}

func (s *Store) Get(key string) (Flag, bool) {
	return Flag{}, false
}

func (s *Store) Update(key string, f Flag) (Flag, bool) {
	return Flag{}, false
}

func (s *Store) Delete(key string) bool {
	return false
}

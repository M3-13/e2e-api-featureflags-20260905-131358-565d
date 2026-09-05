package main

import (
	"errors"
	"strings"
	"sync"
	"testing"
)

func TestStoreCreateValidation(t *testing.T) {
	tests := []struct {
		name    string
		flag    Flag
		wantErr error
	}{
		{"empty key", Flag{Key: "", Enabled: true}, ErrEmptyKey},
		{"key too long", Flag{Key: strings.Repeat("a", 201), Enabled: true}, ErrKeyTooLong},
		{"key exactly 200 ok", Flag{Key: strings.Repeat("a", 200), Enabled: true}, nil},
		{"rollout negative", Flag{Key: "a", RolloutPercent: -1}, ErrRolloutRange},
		{"rollout too high", Flag{Key: "a", RolloutPercent: 101}, ErrRolloutRange},
		{"rollout zero ok", Flag{Key: "a", RolloutPercent: 0}, nil},
		{"rollout 100 ok", Flag{Key: "a", RolloutPercent: 100}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewStore()
			err := s.Create(tt.flag)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("Create() = %v, want nil", err)
				}
				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create() = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestStoreCreateDuplicate(t *testing.T) {
	s := NewStore()
	if err := s.Create(Flag{Key: "dup"}); err != nil {
		t.Fatalf("first Create() = %v", err)
	}
	if err := s.Create(Flag{Key: "dup"}); !errors.Is(err, ErrFlagExists) {
		t.Fatalf("second Create() = %v, want ErrFlagExists", err)
	}
}

func TestStoreListSortedStableCopy(t *testing.T) {
	s := NewStore()
	keys := []string{"beta", "alpha", "gamma"}
	for _, k := range keys {
		if err := s.Create(Flag{Key: k}); err != nil {
			t.Fatalf("Create(%q) = %v", k, err)
		}
	}

	flags := s.List()
	if len(flags) != 3 {
		t.Fatalf("List() len = %d, want 3", len(flags))
	}
	want := []string{"alpha", "beta", "gamma"}
	for i, k := range want {
		if flags[i].Key != k {
			t.Fatalf("List()[%d].Key = %q, want %q", i, flags[i].Key, k)
		}
	}

	// Mutating the returned slice must not affect the store.
	flags[0].Key = "corrupted"
	again := s.List()
	if again[0].Key != "alpha" {
		t.Fatalf("List() returned a non-copy slice; store mutated")
	}
}

func TestStoreGet(t *testing.T) {
	s := NewStore()
	if err := s.Create(Flag{Key: "present", Enabled: true, Description: "desc", RolloutPercent: 42}); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	f, ok := s.Get("present")
	if !ok {
		t.Fatal("Get() ok = false, want true")
	}
	if f.Key != "present" || !f.Enabled || f.Description != "desc" || f.RolloutPercent != 42 {
		t.Fatalf("Get() = %+v, want matching flag", f)
	}

	if _, ok := s.Get("missing"); ok {
		t.Fatal("Get(missing) ok = true, want false")
	}
}

func TestStoreUpdate(t *testing.T) {
	s := NewStore()
	if err := s.Create(Flag{Key: "k", Enabled: false, Description: "old", RolloutPercent: 10}); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	updated, ok := s.Update("k", Flag{Key: "k", Enabled: true, Description: "new", RolloutPercent: 55})
	if !ok {
		t.Fatal("Update() ok = false, want true")
	}
	if !updated.Enabled || updated.Description != "new" || updated.RolloutPercent != 55 {
		t.Fatalf("Update() = %+v, want updated fields", updated)
	}

	got, _ := s.Get("k")
	if !got.Enabled || got.Description != "new" || got.RolloutPercent != 55 {
		t.Fatalf("Get() after update = %+v, want persisted changes", got)
	}

	if _, ok := s.Update("missing", Flag{Key: "missing"}); ok {
		t.Fatal("Update(missing) ok = true, want false")
	}

	// Invalid rollout must not update.
	if _, ok := s.Update("k", Flag{Key: "k", RolloutPercent: -5}); ok {
		t.Fatal("Update with invalid rollout ok = true, want false")
	}
	got, _ = s.Get("k")
	if got.RolloutPercent != 55 {
		t.Fatalf("rollout changed to %d despite invalid update", got.RolloutPercent)
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore()
	if err := s.Create(Flag{Key: "k"}); err != nil {
		t.Fatalf("Create() = %v", err)
	}

	if !s.Delete("k") {
		t.Fatal("Delete() = false, want true")
	}
	if _, ok := s.Get("k"); ok {
		t.Fatal("Get() after delete ok = true, want false")
	}
	if s.Delete("k") {
		t.Fatal("second Delete() = true, want false")
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			s.Create(Flag{Key: string(rune('a'+n%26)) + string(rune('a'+n%26)) + string(rune('0'+n%10))})
			s.List()
			s.Get("some-key")
			s.Update("some-key", Flag{Key: "some-key", RolloutPercent: 50})
			s.Delete("some-key")
		}(i)
	}
	wg.Wait()
}

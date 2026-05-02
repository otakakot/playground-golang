package main_test

import "testing"

func TestTestingCleanUp(t *testing.T) {
	t.Cleanup(func() {
		t.Log("cleanup")
	})
	t.Run("test", func(t *testing.T) {
		t.Log("test")
		t.Run("subtest", func(t *testing.T) {
			t.Fatal("subtest")
		})
	})
}

func TestDefer(t *testing.T) {
	defer func() {
		t.Log("defer")
	}()
	t.Run("test", func(t *testing.T) {
		t.Log("test")
		t.Run("subtest", func(t *testing.T) {
			t.Fatal("subtest")
		})
	})
}

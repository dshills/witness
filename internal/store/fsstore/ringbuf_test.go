package fsstore

import (
	"fmt"
	"testing"
)

func TestRingBuffer_AddAndContains(t *testing.T) {
	r := newRingBuffer(4)

	r.Add("a")
	r.Add("b")
	r.Add("c")

	if !r.Contains("a") {
		t.Error("should contain a")
	}
	if !r.Contains("b") {
		t.Error("should contain b")
	}
	if r.Contains("d") {
		t.Error("should not contain d")
	}
}

func TestRingBuffer_Eviction(t *testing.T) {
	r := newRingBuffer(3)

	r.Add("a")
	r.Add("b")
	r.Add("c")
	// Buffer full, next add evicts "a"
	r.Add("d")

	if r.Contains("a") {
		t.Error("a should have been evicted")
	}
	if !r.Contains("b") {
		t.Error("should contain b")
	}
	if !r.Contains("d") {
		t.Error("should contain d")
	}
}

func TestRingBuffer_FullCycle(t *testing.T) {
	size := 256
	r := newRingBuffer(size)

	for i := range size + 100 {
		r.Add(fmt.Sprintf("evt_%d", i))
	}

	// First 100 should be evicted
	for i := range 100 {
		if r.Contains(fmt.Sprintf("evt_%d", i)) {
			t.Errorf("evt_%d should have been evicted", i)
		}
	}
	// Last 256 should be present
	for i := 100; i < size+100; i++ {
		if !r.Contains(fmt.Sprintf("evt_%d", i)) {
			t.Errorf("evt_%d should be present", i)
		}
	}
}

// Package omap provides an ordered map implementation that maintains insertion order.
package omap

import "iter"

// OrderedMap maintains key-value pairs with insertion order preservation.
type OrderedMap[K comparable, V any] struct {
	order []K
	items map[K]V
}

// New creates an empty OrderedMap.
func New[K comparable, V any]() *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		order: nil,
		items: nil,
	}
}

// NewWithCapacity creates an empty OrderedMap with preallocated capacity.
func NewWithCapacity[K comparable, V any](capacity int) *OrderedMap[K, V] {
	return &OrderedMap[K, V]{
		order: make([]K, 0, capacity),
		items: make(map[K]V, capacity),
	}
}

// Set sets the value for the given key.
// If the key is new, it is appended to the end of the order.
// If the key already exists, its value is updated and the order remains unchanged.
func (m *OrderedMap[K, V]) Set(key K, value V) {
	if m.items == nil {
		m.items = make(map[K]V)
		m.order = []K{}
	}

	if _, exists := m.items[key]; !exists {
		m.order = append(m.order, key)
	}

	m.items[key] = value
}

// Get returns the value associated with the key, and a boolean indicating whether the key exists.
func (m *OrderedMap[K, V]) Get(key K) (V, bool) {
	if m.items == nil {
		var zero V
		return zero, false
	}

	value, ok := m.items[key]

	return value, ok
}

// Has returns true if the key exists in the map.
func (m *OrderedMap[K, V]) Has(key K) bool {
	if m.items == nil {
		return false
	}

	_, ok := m.items[key]

	return ok
}

// Delete removes the key-value pair from the map.
// It returns true if the key was present and removed.
func (m *OrderedMap[K, V]) Delete(key K) bool {
	if m.items == nil {
		return false
	}

	if _, exists := m.items[key]; !exists {
		return false
	}

	delete(m.items, key)

	// Remove from order.
	for i, k := range m.order {
		if k == key {
			m.order = append(m.order[:i], m.order[i+1:]...)
			break
		}
	}

	return true
}

// Len returns the number of key-value pairs in the map.
func (m *OrderedMap[K, V]) Len() int {
	if m.items == nil {
		return 0
	}

	return len(m.items)
}

// Keys returns an iterator over keys in insertion order.
func (m *OrderedMap[K, V]) Keys() iter.Seq[K] {
	return func(yield func(K) bool) {
		for _, key := range m.order {
			if _, ok := m.items[key]; ok {
				if !yield(key) {
					return
				}
			}
		}
	}
}

// Values returns an iterator over values in insertion order.
func (m *OrderedMap[K, V]) Values() iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, key := range m.order {
			if value, ok := m.items[key]; ok {
				if !yield(value) {
					return
				}
			}
		}
	}
}

// Items returns an iterator over key-value pairs in insertion order.
func (m *OrderedMap[K, V]) Items() iter.Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for _, key := range m.order {
			if value, ok := m.items[key]; ok {
				if !yield(key, value) {
					return
				}
			}
		}
	}
}

// Clear removes all key-value pairs from the map.
func (m *OrderedMap[K, V]) Clear() {
	m.order = nil
	m.items = nil
}

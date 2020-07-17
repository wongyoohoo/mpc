//
// Copyright (c) 2020 Markku Rossi
//
// All rights reserved.
//

package ssa

import (
	"sort"
)

// Set implements variable set
type Set map[VariableID]Variable

// NewSet creates a new string value set.
func NewSet() Set {
	return make(map[VariableID]Variable)
}

// Contains tests if the argument value exists in the set.
func (set Set) Contains(val VariableID) bool {
	_, ok := set[val]
	return ok
}

// Add adds a value to the set.
func (set Set) Add(val Variable) {
	set[val.ID] = val
}

// Remove removes a value from set set. The operation does nothing if
// the value did not exist in the set.
func (set Set) Remove(val Variable) {
	delete(set, val.ID)
}

// Copy creates a copy of the set.
func (set Set) Copy() Set {
	result := make(map[VariableID]Variable)
	for k, v := range set {
		result[k] = v
	}
	return result
}

// Subtract removes the values of the argument set from the set.
func (set Set) Subtract(o Set) {
	for _, v := range o {
		set.Remove(v)
	}
}

// Array returns the values of the set as an array.
func (set Set) Array() []Variable {
	var result []Variable
	for _, v := range set {
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

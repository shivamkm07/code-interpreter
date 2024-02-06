package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Sum(x int, y int) int {
	return x + y
}

func TestSum(t *testing.T) {
	total := Sum(5, 5)
	if total != 10 {
		t.Errorf("Sum was incorrect, got: %d, want: %d.", total, 10)
	}

	// Assert that the sum of 5 and 5 is 10
	assert.Equal(t, 10, total, "Sum was correct")
}

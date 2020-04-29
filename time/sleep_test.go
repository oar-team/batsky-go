// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"fmt"
	"testing"
)

func TestSimpleTimer(t *testing.T) {
	fmt.Println("\nSimple timer")
	timer := NewTimer(Second)
	fmt.Printf("Simple timer fired, %v\n", <-timer.C)
}

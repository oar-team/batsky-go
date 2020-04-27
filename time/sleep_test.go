// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package time

import (
	"fmt"
	"testing"

	. "github.com/oar-team/batsky-go/time"
)

func TestSimpleTimer(t *testing.T) {
	timer := NewTimer(Second)
	<-timer.C
	fmt.Println("Simple timer fired")
}

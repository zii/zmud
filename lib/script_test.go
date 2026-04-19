package lib

import (
	"testing"
)

func Test1(t *testing.T) {
	d := totalWaitDuration([]string{"climb tree", "#wa 2s"})
	t.Log("d:", d)
}

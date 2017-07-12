package mem

import (
	"github.com/cloudfoundry/gosigar"
)

// Mem wraps sigar.Mem to provide a Util field for a calculated memory
// utilization
type Mem struct {
	*sigar.Mem

	Util uint64
}

func New() *Mem {
	return &Mem{
		Mem: &sigar.Mem{},
	}
}

func (m *Mem) Get() (err error) {
	if err = m.Mem.Get(); err != nil {
		return
	}

	// calculate memory utilization
	if m.Mem.Total > 0 {
		m.Util = 100 * m.Mem.Used / m.Mem.Total
	}

	return
}

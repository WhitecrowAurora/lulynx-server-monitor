package center

type minuteBucket struct {
	minute int64
	rx     uint64
	tx     uint64
}

type globalRolling struct {
	buckets []minuteBucket
}

func newGlobalRolling(size int) *globalRolling {
	if size < 60 {
		size = 60
	}
	return &globalRolling{buckets: make([]minuteBucket, size)}
}

func (g *globalRolling) Add(tsMS int64, rxBytes, txBytes uint64) {
	if tsMS <= 0 {
		return
	}
	minute := tsMS / 60000
	idx := int(minute % int64(len(g.buckets)))
	b := &g.buckets[idx]
	if b.minute != minute {
		*b = minuteBucket{minute: minute}
	}
	b.rx += rxBytes
	b.tx += txBytes
}

func (g *globalRolling) SumLastMinutes(nowMS int64, minutes int64) (rx uint64, tx uint64) {
	if nowMS <= 0 || minutes <= 0 {
		return 0, 0
	}
	nowMin := nowMS / 60000
	for i := int64(0); i < minutes; i++ {
		m := nowMin - i
		idx := int(m % int64(len(g.buckets)))
		if idx < 0 {
			idx += len(g.buckets)
		}
		b := g.buckets[idx]
		if b.minute == m {
			rx += b.rx
			tx += b.tx
		}
	}
	return rx, tx
}


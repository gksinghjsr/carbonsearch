package full

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/dgryski/carbonzipper/mlog"

	"github.com/kanatohodets/carbonsearch/index"
)

var logger mlog.Level

type Index struct {
	index atomic.Value //map[index.Tag][]index.Metric

	// reporting
	readableTags   uint32
	generation     uint64
	generationTime int64 // time.Duration
}

func NewIndex() *Index {
	fi := &Index{}
	fi.index.Store(make(map[index.Tag][]index.Metric))

	return fi
}

// Materialize should panic in case of any problems with the data -- that
// should have been caught by validation before going into the write buffer
func (fi *Index) Materialize(wg *sync.WaitGroup, fullBuffer map[index.Tag]map[index.Metric]struct{}) {
	defer wg.Done()
	start := time.Now()

	fullIndex := make(map[index.Tag][]index.Metric)
	var readableTags uint32
	for tag, metricSet := range fullBuffer {
		readableTags++
		for metric, _ := range metricSet {
			fullIndex[tag] = append(fullIndex[tag], metric)
		}
	}

	for _, metricList := range fullIndex {
		index.SortMetrics(metricList)
	}

	fi.index.Store(fullIndex)

	// update stats
	fi.SetReadableTags(readableTags)
	fi.IncrementGeneration()

	g := fi.Generation()
	elapsed := time.Since(start)
	fi.IncreaseGenerationTime(int64(elapsed))
	if index.Debug {
		logger.Logf("full index: New generation %v took %v to generate", g, elapsed)
	}
}

func (fi *Index) Query(q *index.Query) ([]index.Metric, error) {
	in := fi.Index()

	metricSets := make([][]index.Metric, 0, len(q.Hashed))
	for _, tag := range q.Hashed {
		metrics, ok := in[tag]
		if !ok {
			return []index.Metric{}, nil
		}

		metricSets = append(metricSets, metrics)
	}

	return index.IntersectMetrics(metricSets), nil
}

func (fi *Index) Index() map[index.Tag][]index.Metric {
	return fi.index.Load().(map[index.Tag][]index.Metric)
}

func (fi *Index) Name() string {
	return "full index"
}

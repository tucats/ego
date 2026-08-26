// Package stats accumulates request outcomes (latency, success/failure) for
// a load run and merges the results across parallel streams.
//
// Latencies are tracked in a fixed set of buckets rather than as raw samples,
// so memory use stays constant no matter how long a run lasts or how many
// requests it makes. Percentiles are estimated by linear interpolation
// within the bucket that contains them -- adequate for a load-exerciser
// summary, not a substitute for a real APM histogram.
package stats

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

// bucketBoundsMS is the upper bound, in milliseconds, of each latency
// bucket. The final bound is +Inf so every observation lands somewhere.
var bucketBoundsMS = []float64{
	1, 2, 5, 10, 20, 50, 100, 200, 500,
	1000, 2000, 5000, 10000, 20000, 50000,
	math.Inf(1),
}

// maxFailureSamples caps how many distinct error messages a summary keeps,
// so a run with many different failure causes doesn't grow unbounded.
const maxFailureSamples = 8

// FailureSample records one distinct error message and how many times it occurred.
type FailureSample struct {
	Message string `json:"message"`
	Count   int64  `json:"count"`
}

// Summary is the serializable result of one stream's run. It is written to
// the --stats-out file when a stream finishes, and merged across streams
// by the orchestrator into an aggregate report.
type Summary struct {
	Stream     int             `json:"stream"`
	Iterations int64           `json:"iterations"`
	Requests   int64           `json:"requests"`
	Successes  int64           `json:"successes"`
	Failures   int64           `json:"failures"`
	StartedAt  time.Time       `json:"started_at"`
	EndedAt    time.Time       `json:"ended_at"`
	Buckets    []int64         `json:"buckets"`
	Samples    []FailureSample `json:"failure_samples,omitempty"`
}

// Collector accumulates request outcomes for one stream during a load run.
// A nil *Collector is valid and every method on it is a no-op, so callers
// outside of load mode can pass a nil collector through unconditionally.
type Collector struct {
	mu      sync.Mutex
	summary Summary
}

// New creates a Collector for the given stream index (0 when not running
// under --parallel).
func New(stream int) *Collector {
	return &Collector{
		summary: Summary{
			Stream:    stream,
			StartedAt: time.Now(),
			Buckets:   make([]int64, len(bucketBoundsMS)),
		},
	}
}

// Record accounts for one completed test execution: its wall-clock duration
// and the error (if any) it produced.
func (c *Collector) Record(d time.Duration, err error) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.summary.Requests++

	ms := float64(d) / float64(time.Millisecond)

	for i, bound := range bucketBoundsMS {
		if ms <= bound {
			c.summary.Buckets[i]++
			break
		}
	}

	if err != nil {
		c.summary.Failures++
		c.recordFailureLocked(err.Error())
	} else {
		c.summary.Successes++
	}
}

func (c *Collector) recordFailureLocked(msg string) {
	for i := range c.summary.Samples {
		if c.summary.Samples[i].Message == msg {
			c.summary.Samples[i].Count++

			return
		}
	}

	if len(c.summary.Samples) < maxFailureSamples {
		c.summary.Samples = append(c.summary.Samples, FailureSample{Message: msg, Count: 1})
	}
}

// IterationDone marks the completion of one full pass through the test suite.
func (c *Collector) IterationDone() {
	if c == nil {
		return
	}

	c.mu.Lock()
	c.summary.Iterations++
	c.mu.Unlock()
}

// Finish stamps the end time and returns the final summary.
func (c *Collector) Finish() Summary {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.summary.EndedAt = time.Now()

	return c.summary
}

// WriteFile serializes the collector's final summary as JSON to path.
func (c *Collector) WriteFile(path string) error {
	summary := c.Finish()

	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, b, 0o644)
}

// LoadFile reads back a Summary previously written by (*Collector).WriteFile.
func LoadFile(path string) (Summary, error) {
	var summary Summary

	b, err := os.ReadFile(path)
	if err != nil {
		return summary, err
	}

	err = json.Unmarshal(b, &summary)

	return summary, err
}

// Merge combines the per-stream summaries produced by --parallel into one
// aggregate. Bucket histograms merge by summing corresponding buckets.
func Merge(summaries []Summary) Summary {
	merged := Summary{Buckets: make([]int64, len(bucketBoundsMS))}

	for _, s := range summaries {
		merged.Iterations += s.Iterations
		merged.Requests += s.Requests
		merged.Successes += s.Successes
		merged.Failures += s.Failures

		for i := range merged.Buckets {
			if i < len(s.Buckets) {
				merged.Buckets[i] += s.Buckets[i]
			}
		}

		if !s.StartedAt.IsZero() && (merged.StartedAt.IsZero() || s.StartedAt.Before(merged.StartedAt)) {
			merged.StartedAt = s.StartedAt
		}

		if s.EndedAt.After(merged.EndedAt) {
			merged.EndedAt = s.EndedAt
		}

		for _, sample := range s.Samples {
			merged.addSample(sample.Message, sample.Count)
		}
	}

	return merged
}

func (s *Summary) addSample(msg string, count int64) {
	for i := range s.Samples {
		if s.Samples[i].Message == msg {
			s.Samples[i].Count += count

			return
		}
	}

	if len(s.Samples) < maxFailureSamples {
		s.Samples = append(s.Samples, FailureSample{Message: msg, Count: count})
	}
}

// Percentile estimates the latency, in milliseconds, at percentile p (0-100)
// by linear interpolation across the bucket histogram. An observation that
// falls in the open-ended final bucket is reported at the last finite
// bound, since there is no upper edge to interpolate against.
func (s Summary) Percentile(p float64) float64 {
	if s.Requests == 0 {
		return 0
	}

	target := int64(math.Ceil((p / 100) * float64(s.Requests)))

	var (
		cumulative int64
		prevBound  float64
	)

	for i, count := range s.Buckets {
		cumulative += count

		if cumulative >= target {
			bound := bucketBoundsMS[i]
			if math.IsInf(bound, 1) || count == 0 {
				return prevBound
			}

			fraction := float64(count-(cumulative-target)) / float64(count)

			return prevBound + fraction*(bound-prevBound)
		}

		prevBound = bucketBoundsMS[i]
	}

	return prevBound
}

// Report renders a human-readable rendering of the summary.
func (s Summary) Report() string {
	var b strings.Builder

	elapsed := s.EndedAt.Sub(s.StartedAt)
	if elapsed <= 0 {
		elapsed = time.Nanosecond
	}

	rps := float64(s.Requests) / elapsed.Seconds()

	errRate := 0.0
	if s.Requests > 0 {
		errRate = 100 * float64(s.Failures) / float64(s.Requests)
	}

	fmt.Fprintf(&b, "LOAD SUMMARY\n")
	fmt.Fprintf(&b, "  duration      %v\n", elapsed.Round(time.Millisecond))
	fmt.Fprintf(&b, "  iterations    %d\n", s.Iterations)
	fmt.Fprintf(&b, "  requests      %d (%.1f req/s)\n", s.Requests, rps)
	fmt.Fprintf(&b, "  successes     %d\n", s.Successes)
	fmt.Fprintf(&b, "  failures      %d (%.2f%%)\n", s.Failures, errRate)
	fmt.Fprintf(&b, "  latency p50   %.1fms\n", s.Percentile(50))
	fmt.Fprintf(&b, "  latency p90   %.1fms\n", s.Percentile(90))
	fmt.Fprintf(&b, "  latency p99   %.1fms\n", s.Percentile(99))

	if len(s.Samples) > 0 {
		fmt.Fprintf(&b, "  sample errors:\n")

		sort.Slice(s.Samples, func(i, j int) bool { return s.Samples[i].Count > s.Samples[j].Count })

		for _, sample := range s.Samples {
			fmt.Fprintf(&b, "    (%d) %s\n", sample.Count, sample.Message)
		}
	}

	return b.String()
}

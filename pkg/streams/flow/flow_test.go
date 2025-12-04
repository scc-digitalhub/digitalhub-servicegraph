// SPDX-FileCopyrightText: © 2025 DSLab - Fondazione Bruno Kessler
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams"
	ext "github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/extension"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/flow"
	"github.com/scc-digitalhub/digitalhub-servicegraph/pkg/streams/internal/assert"
)

func ptr[T any](value T) *T {
	return &value
}

func ptrSlice[T any](slice []T) []*T {
	result := make([]*T, len(slice))
	for i, e := range slice {
		result[i] = ptr(e)
	}
	return result
}

func ptrInnerSlice[T any](slice [][]T) [][]*T {
	outer := make([][]*T, len(slice))
	for i, s := range slice {
		inner := make([]*T, len(s))
		for j, e := range s {
			inner[j] = ptr(e)
		}
		outer[i] = inner
	}
	return outer
}

var addAsterisk = func(in string) []string {
	resultSlice := make([]string, 2)
	resultSlice[0] = in + "*"
	resultSlice[1] = in + "**"
	return resultSlice
}

var filterNotContainsA = func(in string) bool {
	return !strings.ContainsAny(in, "aA")
}

var mtx sync.Mutex

func ingestSlice[T any](source []T, in chan any) {
	mtx.Lock()
	defer mtx.Unlock()
	for _, e := range source {
		in <- e
	}
}

func ingestDeferred[T any](item T, in chan any, wait time.Duration) {
	time.Sleep(wait)
	mtx.Lock()
	defer mtx.Unlock()
	in <- item
}

func closeDeferred[T any](in chan T, wait time.Duration) {
	time.Sleep(wait)
	mtx.Lock()
	defer mtx.Unlock()
	close(in)
}

func readSlice[T any](ch <-chan any) []T {
	var result []T
	for e := range ch {
		result = append(result, e.(T))
	}
	return result
}

func readSlicePtr[T any](ch <-chan any) []*T {
	var result []*T
	for e := range ch {
		result = append(result, e.(*T))
	}
	return result
}

func TestComplexFlow(t *testing.T) {
	in := make(chan any)
	out := make(chan any)

	source := ext.NewChanSource(in)
	toUpperMapFlow := flow.NewMap(strings.ToUpper, 1)
	appendAsteriskFlatMapFlow := flow.NewFlatMap(addAsterisk, 1)
	throttler := flow.NewThrottler(10, 200*time.Millisecond, 50, flow.Backpressure)
	tumblingWindow := flow.NewTumblingWindow[string](200 * time.Millisecond)
	filterFlow := flow.NewFilter(filterNotContainsA, 1)
	sink := ext.NewChanSink(out)

	inputValues := []string{"a", "b", "c"}
	go ingestSlice(inputValues, in)
	go closeDeferred(in, time.Second)

	go func() {
		source.
			Via(toUpperMapFlow).
			Via(flow.NewPassThrough()).
			Via(appendAsteriskFlatMapFlow).
			Via(tumblingWindow).
			Via(flow.Flatten[string](1)).
			Via(throttler).
			Via(filterFlow).
			To(sink)
	}()

	outputValues := readSlice[string](sink.Out)
	expectedValues := []string{"B*", "B**", "C*", "C**"}

	assert.Equal(t, expectedValues, outputValues)
}

func TestSplitFlow(t *testing.T) {
	in := make(chan any, 3)
	out := make(chan any, 3)

	source := ext.NewChanSource(in)
	toUpperMapFlow := flow.NewMap(strings.ToUpper, 1)
	sink := ext.NewChanSink(out)

	inputValues := []string{"a", "b", "c"}
	ingestSlice(inputValues, in)
	close(in)

	split := flow.Split(
		source.
			Via(toUpperMapFlow), filterNotContainsA)

	flow.Merge(split[0], split[1]).
		To(sink)

	outputValues := readSlice[string](sink.Out)
	sort.Strings(outputValues)
	expectedValues := []string{"A", "B", "C"}

	assert.Equal(t, expectedValues, outputValues)
}

func TestSplitFlow_Ptr(t *testing.T) {
	in := make(chan any, 3)
	out := make(chan any, 3)

	source := ext.NewChanSource(in)
	toUpperMapFlow := flow.NewMap(func(s *string) *string {
		upper := strings.ToUpper(*s)
		return &upper
	}, 1)
	sink := ext.NewChanSink(out)

	inputValues := ptrSlice([]string{"a", "b", "c"})
	ingestSlice(inputValues, in)
	close(in)

	split := flow.Split(
		source.Via(toUpperMapFlow),
		func(in *string) bool {
			return !strings.ContainsAny(*in, "aA")
		})

	flow.Merge(split[0], split[1]).
		To(sink)

	var outputValues []string
	for e := range sink.Out {
		v := e.(*string)
		outputValues = append(outputValues, *v)
	}
	sort.Strings(outputValues)
	expectedValues := []string{"A", "B", "C"}

	assert.Equal(t, expectedValues, outputValues)
}

func TestFanOutFlow(t *testing.T) {
	in := make(chan any)
	out := make(chan any)

	source := ext.NewChanSource(in)
	filterFlow := flow.NewFilter(filterNotContainsA, 1)
	toUpperMapFlow := flow.NewMap(strings.ToUpper, 1)
	sink := ext.NewChanSink(out)

	inputValues := []string{"a", "b", "c"}
	go ingestSlice(inputValues, in)
	go closeDeferred(in, 100*time.Millisecond)

	go func() {
		fanOut := flow.FanOut(
			source.
				Via(filterFlow).
				Via(toUpperMapFlow), 2)
		flow.
			Merge(fanOut...).
			To(sink)
	}()

	outputValues := readSlice[string](sink.Out)
	sort.Strings(outputValues)
	expectedValues := []string{"B", "B", "C", "C"}

	assert.Equal(t, expectedValues, outputValues)
}

func TestRoundRobinFlow(t *testing.T) {
	in := make(chan any)
	out := make(chan any)

	source := ext.NewChanSource(in)
	filterFlow := flow.NewFilter(filterNotContainsA, 1)
	toUpperMapFlow := flow.NewMap(strings.ToUpper, 1)
	sink := ext.NewChanSink(out)

	inputValues := []string{"a", "b", "c"}
	go ingestSlice(inputValues, in)
	go closeDeferred(in, 100*time.Millisecond)

	go func() {
		roundRobin := flow.RoundRobin(
			source.
				Via(filterFlow).
				Via(toUpperMapFlow), 2)
		flow.
			Merge(roundRobin...).
			To(sink)
	}()

	outputValues := readSlice[string](sink.Out)
	sort.Strings(outputValues)
	expectedValues := []string{"B", "C"}

	assert.Equal(t, expectedValues, outputValues)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name        string
		flattenFlow streams.Flow
		ptr         bool
	}{
		{
			name:        "values",
			flattenFlow: flow.Flatten[int](1),
			ptr:         false,
		},
		{
			name:        "pointers",
			flattenFlow: flow.Flatten[*int](1),
			ptr:         true,
		},
	}
	input := [][]int{{1, 2, 3}, {4, 5}}
	expected := []int{1, 2, 3, 4, 5}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := make(chan any, 5)
			out := make(chan any, 5)

			source := ext.NewChanSource(in)
			sink := ext.NewChanSink(out)

			if tt.ptr {
				ingestSlice(ptrInnerSlice(input), in)
			} else {
				ingestSlice(input, in)
			}
			close(in)

			source.
				Via(tt.flattenFlow).
				To(sink)

			if tt.ptr {
				output := readSlicePtr[int](out)
				assert.Equal(t, ptrSlice(expected), output)
			} else {
				output := readSlice[int](out)
				assert.Equal(t, expected, output)
			}
		})
	}
}

func TestZipWith(t *testing.T) {
	tests := []struct {
		name        string
		outlets     []streams.Flow
		shouldPanic bool
		expected    []string
	}{
		{
			name:        "no-outlets",
			shouldPanic: true,
		},
		{
			name:        "one-outlet",
			outlets:     []streams.Flow{chanSource([]int{1})},
			shouldPanic: true,
		},
		{
			name:        "wrong-data-type",
			outlets:     []streams.Flow{chanSource([]string{"a"})},
			shouldPanic: true,
		},
		{
			name:     "empty-outlets",
			outlets:  []streams.Flow{chanSource([]int{}), chanSource([]int{})},
			expected: []string{},
		},
		{
			name: "equal-length",
			outlets: []streams.Flow{chanSource([]int{1, 2, 3}),
				chanSource([]int{1, 2, 3}), chanSource([]int{1, 2, 3})},
			expected: []string{"[1 1 1]", "[2 2 2]", "[3 3 3]"},
		},
		{
			name:     "first-longer",
			outlets:  []streams.Flow{chanSource([]int{1, 2, 3}), chanSource([]int{1})},
			expected: []string{"[1 1]", "[2 0]", "[3 0]"},
		},
		{
			name: "second-longer",
			outlets: []streams.Flow{chanSource([]int{1, 2}),
				chanSource([]int{1, 2, 3, 4, 5})},
			expected: []string{"[1 1]", "[2 2]", "[0 3]", "[0 4]", "[0 5]"},
		},
		{
			name: "mixed-length",
			outlets: []streams.Flow{chanSource([]int{1, 2}),
				chanSource([]int{1, 2, 3, 4, 5}), chanSource([]int{1, 2, 3})},
			expected: []string{"[1 1 1]", "[2 2 2]", "[0 3 3]", "[0 4 0]", "[0 5 0]"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				assert.Panics(t, func() {
					flow.ZipWith(func(zipped []int) string {
						return fmt.Sprintf("%v", zipped)
					}, tt.outlets...)
				})
			} else {
				sink := ext.NewChanSink(make(chan any, len(tt.expected)))
				flow.ZipWith(func(zipped []int) string {
					return fmt.Sprintf("%v", zipped)
				}, tt.outlets...).To(sink)

				actual := make([]string, 0, len(tt.expected))
				for e := range sink.Out {
					actual = append(actual, e.(string))
				}

				assert.Equal(t, tt.expected, actual)
			}
		})
	}
}

func TestZipWith_TwoOutlets_ProducesCombined(t *testing.T) {
	sink := ext.NewChanSink(make(chan any, 2))
	flow.ZipWith(func(zipped []int) string { return fmt.Sprintf("%v", zipped) }, chanSource([]int{1, 2}), chanSource([]int{10, 20})).To(sink)
	var out []string
	for e := range sink.Out {
		out = append(out, e.(string))
	}
	if len(out) != 2 || out[0] != "[1 10]" || out[1] != "[2 20]" {
		t.Fatalf("unexpected zip output: %v", out)
	}
}

func TestZipWith_ThreeOutlets_TailShorter_ContainsZero(t *testing.T) {
	sink := ext.NewChanSink(make(chan any, 10))
	flow.ZipWith(func(zipped []int) string { return fmt.Sprintf("%v", zipped) }, chanSource([]int{1, 2, 3}), chanSource([]int{10}), chanSource([]int{100, 200, 300})).To(sink)
	var out []string
	for e := range sink.Out {
		out = append(out, e.(string))
	}
	// at least one combined value should contain a zero for the short middle outlet
	foundZero := false
	for _, s := range out {
		if strings.Contains(s, " 0 ") || strings.Contains(s, "[0 ") || strings.Contains(s, " 0]") {
			foundZero = true
			break
		}
	}
	if !foundZero {
		t.Fatalf("expected at least one zipped output to contain zero, got: %v", out)
	}
}

func TestZipWith_PanicOnInsufficientOutlets(t *testing.T) {
	assert.Panics(t, func() {
		flow.ZipWith(func(z []int) string { return fmt.Sprintf("%v", z) })
	})
}

func chanSource[T any](data []T) streams.Flow {
	ch := make(chan any, len(data))
	for _, value := range data {
		ch <- value
	}
	close(ch)
	return ext.NewChanSource(ch).Via(flow.NewPassThrough())
}

// Merged util tests
func TestMergeFunctionByNameAndConcat_StringAndBytesAndEvents_Merged(t *testing.T) {
	// MergeFunctionByName
	f := flow.MergeFunctionFromSpec(map[string]any{"merge_mode": "concat"})
	if f == nil {
		t.Fatalf("expected concat function for name concat")
	}
	if flow.MergeFunctionFromSpec(map[string]any{"merge_mode": "unknown"}) != nil {
		t.Fatalf("expected nil for unknown merge function")
	}

	// Concat empty
	ev := flow.Concat([]interface{}{})
	if string(ev.GetBody()) != "" {
		t.Fatalf("expected empty body for empty values, got %s", string(ev.GetBody()))
	}

	// Concat strings
	ev = flow.Concat([]interface{}{"a", "b", "c"})
	if string(ev.GetBody()) != "abc" {
		t.Fatalf("expected abc got %s", string(ev.GetBody()))
	}

	// Concat []byte
	ev = flow.Concat([]interface{}{[]byte("x"), []byte("y")})
	if string(ev.GetBody()) != "xy" {
		t.Fatalf("expected xy got %s", string(ev.GetBody()))
	}

	// Concat streams.Event with text/plain
	e1, _ := streams.NewGenericEvent([]byte("t1"), "", "", map[string]string{"content-type": "text/plain"}, nil, 200)
	e2, _ := streams.NewGenericEvent([]byte("t2"), "", "", map[string]string{"content-type": "text/plain"}, nil, 200)
	ev = flow.Concat([]interface{}{e1, e2})
	if string(ev.GetBody()) != "[116 49][116 50]" {
		t.Fatalf("expected numeric representation got %s", string(ev.GetBody()))
	}

	// Concat streams.Event with application/json
	j1, _ := streams.NewGenericEvent([]byte(`{"a":1}`), "", "", map[string]string{"content-type": "application/json"}, nil, 200)
	j2, _ := streams.NewGenericEvent([]byte(`{"b":2}`), "", "", map[string]string{"content-type": "application/json"}, nil, 200)
	ev = flow.Concat([]interface{}{j1, j2})
	if string(ev.GetBody()) != "[{\"a\":1},{\"b\":2}]" {
		t.Fatalf("unexpected merged json: %s", string(ev.GetBody()))
	}

	// Concat streams.Event default (merge bytes)
	d1, _ := streams.NewGenericEvent([]byte("d1"), "", "", nil, nil, 200)
	d2, _ := streams.NewGenericEvent([]byte("d2"), "", "", nil, nil, 200)
	ev = flow.Concat([]interface{}{d1, d2})
	if string(ev.GetBody()) != "d1d2" {
		t.Fatalf("expected d1d2 got %s", string(ev.GetBody()))
	}

	ev = flow.Concat([]interface{}{1, 2})
	if string(ev.GetBody()) != "" {
		t.Fatalf("expected empty body for default concat case, got %s", string(ev.GetBody()))
	}

}

func TestUtilExtra_CoverageHelpers_Merged(t *testing.T) {
	// Split two-way
	in := make(chan any, 3)
	in <- 1
	in <- 2
	in <- 3
	close(in)
	src := ext.NewChanSource(in)
	parts := flow.Split(src, func(i int) bool { return i%2 == 0 })
	var a, b []int
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for e := range parts[0].Out() {
			a = append(a, e.(int))
		}
	}()
	go func() {
		defer wg.Done()
		for e := range parts[1].Out() {
			b = append(b, e.(int))
		}
	}()
	wg.Wait()
	if len(a)+len(b) != 3 {
		t.Fatalf("split produced wrong total count: %d+%d", len(a), len(b))
	}

	// FanOut duplicates to multiple flows
	in2 := make(chan any, 2)
	in2 <- "x"
	in2 <- "y"
	close(in2)
	src2 := ext.NewChanSource(in2)
	outs := flow.FanOut(src2, 2)
	merged := flow.Merge(outs[0], outs[1])
	var mergedValues []string
	for e := range merged.Out() {
		mergedValues = append(mergedValues, e.(string))
	}
	if len(mergedValues) != 4 {
		t.Fatalf("fanout merge expected 4 values, got %d", len(mergedValues))
	}

	// RoundRobin should distribute values
	in3 := make(chan any, 3)
	in3 <- 1
	in3 <- 2
	in3 <- 3
	close(in3)
	src3 := ext.NewChanSource(in3)
	rr := flow.RoundRobin(src3, 2)
	mergedRR := flow.Merge(rr[0], rr[1])
	var rrValues []int
	for e := range mergedRR.Out() {
		rrValues = append(rrValues, e.(int))
	}
	if len(rrValues) != 3 {
		t.Fatalf("roundrobin expected 3 values, got %d", len(rrValues))
	}

	// Merge two simple outlets
	aFlow := chanSource([]int{1, 2})
	bFlow := chanSource([]int{3})
	mergedAB := flow.Merge(aFlow, bFlow)
	var mergedABVals []int
	for e := range mergedAB.Out() {
		mergedABVals = append(mergedABVals, e.(int))
	}
	if len(mergedABVals) != 3 {
		t.Fatalf("merge expected 3 values, got %d", len(mergedABVals))
	}

	// ZipWith simple combine
	sink := ext.NewChanSink(make(chan any, 2))
	flow.ZipWith(func(zipped []int) string { return fmt.Sprintf("%v", zipped) }, chanSource([]int{1}), chanSource([]int{2})).To(sink)
	var zipped []string
	for e := range sink.Out {
		zipped = append(zipped, e.(string))
	}
	if len(zipped) == 0 {
		t.Fatalf("zipwith produced no output")
	}

	// Flatten
	in4 := make(chan any, 2)
	in4 <- []int{1, 2}
	in4 <- []int{3}
	close(in4)
	src4 := ext.NewChanSource(in4)
	flat := flow.Flatten[int](1)
	// attach source first then To to avoid blocking
	src4.Via(flat)
	flat.To(ext.NewChanSink(make(chan any, 3)))

	// SplitMulti out-of-range predicate should be ignored gracefully
	in5 := make(chan any, 1)
	in5 <- 99
	close(in5)
	src5 := ext.NewChanSource(in5)
	partsMulti := flow.SplitMulti(src5, 3, func(in any) int { return 1 })
	if len(partsMulti) != 3 {
		t.Fatalf("splitmulti expected 3 parts, got %d", len(partsMulti))
	}
	// ensure drains
	for _, p := range partsMulti {
		for range p.Out() {
		}
	}
}

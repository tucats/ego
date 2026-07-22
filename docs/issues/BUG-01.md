# BUG-01 — `for v := range ch` over channel yields indices, not values

**Severity:** HIGH

**Description:**  
When ranging over a channel with a single loop variable, the variable receives
successive integer indices (0, 1, 2, …) instead of the values stored in the channel.
The channel values are discarded. This makes the standard Go idiom of consuming a
channel with `for v := range ch` produce completely wrong results.

**Reproducer:**

```go
import "fmt"

func producer(ch chan) {
    ch <- 100
    ch <- 200
    ch <- 300
    close(ch)
}

func main() {
    ch := make(chan, 5)
    go producer(ch)

    for v := range ch {
        fmt.Println("received:", v)
    }
}
```

**Actual output:**

```text
received: 0
received: 1
received: 2
```

**Expected output:**

```text
received: 100
received: 200
received: 300
```

**Notes:**  
Direct receive (`v := <-ch`) returns the correct values. Only the `for range`
form is broken. The bug appears to be that the range loop treats the channel
like an array and emits the iteration count rather than the received value.

**Resolution (June 2026):**  
`bytecode/range.go` — `rangeNextChannel`: the function now distinguishes between
the single-variable and two-variable loop forms by checking whether `r.valueName`
is present:

- **Two-variable form** (`for i, v := range ch`): `r.indexName` receives the loop
  counter (`r.index`) and `r.valueName` receives the channel value (`datum`) —
  behavior unchanged.
- **Single-variable form** (`for v := range ch`): the compiler emits
  `RangeInit ["v", ""]`, placing the sole variable name in `r.indexName` and
  leaving `r.valueName` empty. The function now routes `datum` to `r.indexName`
  in this case. The loop counter is meaningless for channel receives and is
  no longer exposed.

Three new unit tests were added to `bytecode/range_test.go` (Section 8):

- `Test_rangeNextChannel_SingleVarReceivesValue` — the direct regression test;
  verifies that the single-variable form delivers channel values, not indices.
- `Test_rangeNextChannel_SingleVarDiscarded` — verifies the discard form
  (`for _ := range ch`) consumes values without writing to any symbol.
- `Test_rangeNextChannel_TwoVarCounterAndValue` — verifies the two-variable form
  still delivers the counter to `i` and the value to `v` after the fix.

Two new Ego language tests were added to `tests/flow/rangechannels.ego`:

- `"flow: single-variable channel range receives values, not indices"` — the
  end-to-end regression test.
- `"flow: two-variable channel range counter and value are correct"` — ensures
  the two-variable form continues to work.

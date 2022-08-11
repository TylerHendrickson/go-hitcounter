# go-hitcounter

## What is it?

A go library that provides rolling-period hit counters.

Contains a `HitCounter` interface with two implementations:
- `ImmediateHitCounter` used when calls to record a new hit are done so monotonically.
- `FlexibleHitCounter` optimized for recording hits out-of-order at specified times.


## Notes

In both implementations, memory for storing hits is allocated upon creation, and remains
fixed for their entire lifecycle. The amount of memory allocated is determined by the
configured duration and resolution of the hit counter, where the duration is the period
before a hit expires, and the resolution is how often hits are expired. For example,
a hit counter with `(duration=5minutes, resolution=1minute)` will consider hits expired
when they are 5 minutes old, and groups hits together in 1-minute increments. Higher resolution
therefore means more memory, but also grants higher precision when evaluating total hits
within the configured duration.


## Testing

Tests exist for both implementations, although they are duplicative. Run `go test` to execute.


## Todo

- Tests could be more thorough for `FlexibleHitCounter`.
- Improved coverage generally.
- Better code reuse – `flexible.go` essentially copied `immediate.go` and adjusted the `AddHit()`
mechanics, which isn't very elegant.
- Could use some code examples, such as a server that exposes a configured counter.
- README could use a usage demonstration.

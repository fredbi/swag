# Roadmap

* extend json name utility to support untagged, exported fields

* extend json adapters to support go-ccy json serialization library

* update benchmarks: go1.25 (possibly go1.26) have improved the json write operation (less allocations):
  this changes entirely the comparison with easyjson. Verify / update benchmark / check that extraneous allocs
  are not due to our shim layer.

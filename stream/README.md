# stream
The stream package offers an interface to support a data stream interface fo [gotojs](../). In order to implement a so-called stream [source](source.go) the following methods need to be implemented in order to satisfy the requirements:


| signature | semantics |
| --- | --- | --- |
| ```Start() error``` | Start the stream. This includes connecting to a remote source and start buffering data. |
| ```Close() ``` | Close and stop the stream. Free all resource associated with it. |
| ```Next() (m Message,err error)``` |  REtrieve the next message from the data stream. |

Its up to the implementing package how to instantiate the source or directly the [stream](stream.go). Conventually those are named like :
```go
func NewFooBarStream(...) (s *Stream,err error) {... NewStream(...) ...}
func NewFooBarSource(...) (s *Source,err error) {...}
```
As an example please see the [twitter](twitter/twitter.go) implementation.

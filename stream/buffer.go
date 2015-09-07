package stream

import (
	"log"
	"sync"
	"time"
)

type Timestamp uint64 //msecs
type ID uint64

const (
	DefaultBufferSize = 12 //BitSize
	MsecTimeDivisor   = 1000 * 1000
)

type Message struct {
	Id      ID          `json:"id"`
	Time    Timestamp   `json:"time"`
	DTime   Timestamp   `json:"dtime"`
	Payload interface{} `json:"payload"`
}

//New returns a javascript compatible timestamp. (msecs since 1.1.1970, 0am)
func Now() Timestamp {
	return Timestamp(time.Now().UnixNano() / MsecTimeDivisor)
}

//NewMessage instantiates an empty Message with the given payload.
func NewMessage(payload interface{}) Message {
	return Message{Id: 0, Time: Now(), Payload: payload}
}

type Buffer struct {
	cid   ID //next free slot, last slot: buf[cid-1]
	buf   []Message
	mutex sync.Mutex
	mask  ID
	size  ID
}

//NewBuffer initializes a new message ring-buffer.
func NewBuffer(bsize uint) (buf *Buffer) {
	buf = new(Buffer)
	buf.cid = 0
	buf.size = 1 << (bsize)
	buf.buf = make([]Message, buf.size)
	buf.mask = 0

	for i := bsize; i > 0; i-- {
		buf.mask |= 1 << (i - 1)
	}

	log.Printf("Initialized Buffer: %d/%x", buf.size, buf.mask)
	return
}

//Empty clears the entire buffer. All remaining messages will be discarded.
func (b *Buffer) Empty() *Buffer {
	b.cid = 0
	b.buf = make([]Message, b.size)
	return b
}

//Enqueue adds a message to the buffer. If the buffer is fully occupied it will be added to the beginning.
//Thus it acts as a ringbuffer.
func (b *Buffer) Enqueue(message Message) *Buffer {
	now := Now()
	b.mutex.Lock()
	id := b.cid & b.mask
	ltime := now
	if id > 0 {
		ltime = b.buf[id-1].Time
	}

	message.Id = b.cid
	message.DTime = now - ltime

	b.cid++
	b.buf[id] = message

	b.mutex.Unlock()
	return b
}

//HasNext checks whether there are more messages in the queue after the given ID.
func (b *Buffer) HasNext(id ID) bool {
	return b.cid > id
}

//Fetch returns a slice of Messages that fit the request criteria.
// If no paramerter is given all available messages will be returned.
// If one ID is given as parameter, all Messages starting from this ID will be returned.
// If two IDs are given all messages between the first and the second will be returned.
func (b *Buffer) Fetch(vals ...ID) (ret []Message) {
	b.mutex.Lock()
	var id, max ID
	id = 0
	max = 0

	if len(vals) > 0 {
		id = vals[0]
	}

	if len(vals) > 1 {
		max = vals[1]
	}

	if b.cid == 0 || id > b.cid {
		b.mutex.Unlock()
		return
	}

	cid := b.cid - 1

	if max > 0 && (id+max) < cid {
		cid = id + (max - 1)
	}

	from := ID(id & b.mask)
	to := ID(cid & b.mask)
	diff := int64(cid) - int64(id)
	mdiff := (int64(from) - int64(to))
	if mdiff < 0 {
		mdiff = int64(to - from)
	}

	if diff > mdiff {
		from = to + 1
	}

	if from > to && cid != 0 {
		a1 := ID(from)
		a2 := ID(b.size - 1)
		b1 := ID(0)
		b2 := ID(to)
		s := (b.size - from) + to + 1
		ret = make([]Message, s)

		copy(ret[0:(a2-a1)+1], b.buf[a1:a2+1])
		copy(ret[(a2-a1)+1:s], b.buf[b1:b2+1])
	} else {
		ret = b.buf[from : to+1]
	}
	b.mutex.Unlock()
	return
}

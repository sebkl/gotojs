// Package stream of GOTOJS offers an interface to expose event or message streams.
// Stream implementations just need to implement the Source interface and define a Message type
// which is encodable as JSON.
package stream

import (
	"container/list"
	"encoding/json"
	"fmt"
	. "github.com/sebkl/gotojs"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	DefaultMaxRecordCount   = 10000 //in count of messages
	DefaultLazyStart        = true
	DefaultRetryCount       = 1
	DefaultMaxRetryDeadline = 5 * time.Second
	DefaultSessionTimeout   = 30 * time.Second
	DefaultRequestTimeout   = 4 * time.Second
	DefaultStopOnLonely     = true
)

type Source interface {
	Next() (Message, error)
	Close()
	Start() error
}

type StreamSession struct {
	Id         ID
	Next       ID
	LastAccess Timestamp
	Begin      Timestamp
}

type FetchRequest struct {
	t   time.Time
	id  ID
	s   *StreamSession
	q   chan []Message
	max ID
}

func NewFetchRequest(s *StreamSession) *FetchRequest {
	return &FetchRequest{
		t:   time.Now(),
		id:  s.Next,
		s:   s,
		max: 0,
		q:   make(chan []Message)}
}

func (f *FetchRequest) TimedOut(d time.Duration) bool { return time.Now().After(f.t.Add(d)) }

type Fetcher struct {
	config   *Configuration
	sessions map[ID]*StreamSession
	source   Source
	buf      *Buffer
	backlog  *list.List
	timeout  time.Duration
	notify   chan int
	running  bool
}

//NewFetcher creates a new fetcher which consits of a BacklogRunner process.
func NewFetcher(source Source, c *Configuration) (ret *Fetcher, err error) {
	ret = &Fetcher{running: false, source: source}
	ret.config = c
	ret.backlog = list.New()
	ret.notify = make(chan int)
	ret.timeout = DefaultRequestTimeout
	ret.sessions = make(map[ID]*StreamSession)

	go func() {
		ret.BacklogRunner()
	}()

	return
}

//Internally used function to determine whether the source stream can be stopped since to more client stream are active.
func (f *Fetcher) cleanup(timeout Timestamp) {
	now := Now()
	for k, v := range f.sessions {
		dts := Timestamp(now - v.LastAccess)
		if dts > timeout {
			log.Printf("Killing session: %d, Timedout since %d seconds.", v.Id, int64(dts/1000))
			delete(f.sessions, k)
		}
	}

	if len(f.sessions) < 1 && f.config.StopOnLonely {
		f.Stop()
	}
}

// Fetch puts a fetchrequest to the backlog queue. The BacklogRunner process is taking care for sending the actual data back to the clients.
func (f *Fetcher) Fetch(fr *FetchRequest) (ret []Message) {
	if f.buf.HasNext(fr.id) {
		ret = f.buf.Fetch(fr.id)
	} else {
		if f.running {
			// Reduce offset to 1 in order to avoid future requests
			fr.id = f.buf.cid + 1
			f.backlog.PushBack(fr)

			ret = <-fr.q // this blocks until new data arrives.
		} else {
			//This locks
			return nil
		}
	}
	return
}

//BacklogRunner is the worker loop that continoously serves the client (fetch requests) with outstanding messages.
func (f *Fetcher) BacklogRunner() {
	log.Printf("Started BacklogRunner")
	for {
		select {
		case _ = <-f.notify:
			for e := f.backlog.Front(); e != nil; e = e.Next() {
				fr, ok := e.Value.(*FetchRequest)
				if ok {

					if f.buf.HasNext(fr.id) {
						fr.q <- f.buf.Fetch(fr.id)
						f.backlog.Remove(e)
					}
				} else {
					log.Printf("Backlog contains invalid element type.")
					f.backlog.Remove(e)
				}
			}
			f.cleanup(Timestamp(DefaultSessionTimeout.Nanoseconds() / MsecTimeDivisor))
		case _ = <-time.After(f.timeout):
			for e := f.backlog.Front(); e != nil; e = e.Next() {
				fr, ok := e.Value.(*FetchRequest)
				if ok && fr.TimedOut(f.timeout) {
					log.Printf("Request timed out for session: %d", fr.s.Id)
					f.backlog.Remove(e)
					fr.q <- nil
				}
			}
		}
	}
	log.Printf("BacklogRunner stopped")
}

//Start starts the fetcher loop. It blocks and takes all incoming messages and enqueues them to the buffer.
//Start returns only if the stream is stopped.
func (f *Fetcher) Start() (err error) {
	ret := make(chan error)
	f.buf = NewBuffer(uint(f.config.BufferBitSize))
	go func(ret chan error) {
		log.Printf("Starting source stream.")
		//Try to start source stream.
		for retryCount := 0; retryCount < DefaultRetryCount; retryCount++ {
			startTime := time.Now()
			if err = f.source.Start(); err != nil {
				log.Printf("Could not start source stream: %s", err)
			}

			if tdif := time.Now().Sub(startTime); tdif > DefaultMaxRetryDeadline {
				retryCount = 0 //reset retry counter
				log.Printf("Reinitiating connection. error within %d seconds. ", tdif)
			}
		}

		if err != nil {
			ret <- err
			return
		}
		//TODO: FORK ROUTINE HERE !!! and return success/failure immediately.
		f.running = true
		ret <- nil
		for f.running {
			message, err := f.source.Next()
			if err == nil {
				f.buf.Enqueue(message)
				f.notify <- 1
			} else {
				log.Printf("Could not fetch next message from source. %s", err)
			}
		}
		log.Printf("Stream has stopped.")
		f.buf.Empty()
	}(ret)
	return <-ret //Wait for routine to be started.
}

// Stop interrupts the source stream. No more data is enqueued to the buffer anymore until Start() is called.
// This method is used when the system identifies that no more clients have subscribed to the stream.
func (f *Fetcher) Stop() {
	if f.running {
		log.Printf("Stopping stream.")
		f.running = false
		f.source.Close()
	}
}

type Configuration struct {
	SessionTimeout Timestamp //TODO: currently not used by fetcher. needs to be moved to fetcher.
	MaxRecordCount int
	BufferBitSize  uint
	LazyStart      bool
	StopOnLonely   bool
}

type Stream struct {
	fetcher *Fetcher
}

//Internally used function to identifiy the users buffer cursor (called Stream) based on its GOTOJS session infromation.
func (t *Stream) session(session *Session) (s *StreamSession) {
	iid, _ := strconv.ParseInt(session.Get("ID"), 10, 64)
	id := ID(iid)

	if _, ok := t.fetcher.sessions[id]; !ok {
		nid := ID(rand.Int63())
		s = &StreamSession{
			Id:    nid,
			Next:  t.fetcher.buf.cid,
			Begin: Now()}
		session.Set("ID", fmt.Sprintf("%d", nid))
		log.Printf("Created with session with id: %d", nid)
		t.fetcher.sessions[nid] = s
	} else {
		s = t.fetcher.sessions[id]
	}
	return s
}

//Method to be exposed for message retrieval. Cursor information is stored in the users GOTOJS session.
//Clients may frequently call this method to retrieve new messages.
func (t *Stream) Next(session *Session, c *HTTPContext) (ret []Message) {
	if t.fetcher.config.LazyStart && !t.fetcher.running {
		log.Printf("Starting stream (lazy start).")
		t.Start()
	}

	s := t.session(session)
	s.LastAccess = Now()
	fr := NewFetchRequest(s)
	fr.max = ID(t.fetcher.config.MaxRecordCount)
	ret = t.fetcher.Fetch(fr)
	if ret != nil {
		s.Next = ret[len(ret)-1].Id + 1 // Update id
	} else {
		c.ReturnStatus = http.StatusNoContent
		log.Printf("No more messages for: '%d'", s.Id)
	}
	return
}

//Reset resets the users session. The cursor will be deleted and reinitialized.
func (t *Stream) Reset(session *Session) {
	s := t.session(session)
	delete(t.fetcher.sessions, s.Id)
}

// Stop stops a stream and purges all open sessions.
func (t *Stream) Stop() {
	t.fetcher.Stop()
	t.fetcher.cleanup(0) //0 indicates to cleanup all active sessions.
}

// Start checks whether the fetcher is currently running. If yes it returns immediately.
// If not it is started. This mechanism allows to stop twitter source stream if no scubscribers
// or sessions are currently open.
func (t *Stream) Start() {
	if !t.fetcher.running {
		log.Printf("Starting stream process.")
		if err := t.fetcher.Start(); err != nil {
			log.Println(err)
		}
	} else {
		log.Printf("Stream already running.")
	}
}

//NewStream creates a new Stream based on the given Source implementation.
//The default configuration can be overwritten by a configuration file named "streamconfig.json"
func NewStream(source Source) (t *Stream, err error) {

	config := &Configuration{
		SessionTimeout: Timestamp(DefaultSessionTimeout.Nanoseconds() / 1000),
		MaxRecordCount: DefaultMaxRecordCount,
		LazyStart:      DefaultLazyStart,
		StopOnLonely:   DefaultStopOnLonely,
		BufferBitSize:  DefaultBufferSize,
	}

	fetcher, err := NewFetcher(source, config) // just pass the source connection here.

	t = &Stream{
		fetcher: fetcher,
	}

	// Load configuration from file
	filename := "streamconfig.json"
	configFile, oerr := os.Open(filename)
	if oerr == nil {
		log.Printf("Loading configuration from %s", filename)
		decoder := json.NewDecoder(configFile)
		err := decoder.Decode(config)
		if err != nil {
			log.Fatal("Could not decode '%s': %s", filename, err)
		}
	} else {
		log.Printf("Could not load configuration from %s", filename)
	}

	// Start fetcher process if no lazy start is configured
	if !config.LazyStart {
		log.Printf("Starting buffer routine.")
		t.Start()
	}
	return
}

// Package stream of GOTOJS offers an interface to expose event or message streams.
// Stream implementations just need to implement the Source interface and define a Message type
// which is encodable as JSON.
package stream

import (
	. "github.com/sebkl/gotojs"
	"log"
	"fmt"
	"math/rand"
	"strconv"
	"os"
	"encoding/json"
)

const (
	DefaultMaxRecordCount = 10000 //in count of messages
	DefaultLazyStart = true
)

type Configuration struct {
	SessionTimeout Timestamp
	MaxRecordCount int
	BufferBitSize int
	LazyStart bool
}

type Stream struct {
	fetcher *Fetcher
	Config Configuration
}


//Internally used function to identifiy the users buffer cursor (called Stream) based on its GOTOJS session infromation.
func (t *Stream) session(session *Session) (s *StreamSession) {
	iid,_ := strconv.ParseInt(session.Get("ID"),10,64)
	id := ID(iid)

	if _,ok := t.fetcher.sessions[id]; !ok {
		nid := ID(rand.Int63())
		s = &StreamSession{
			Id: nid,
			Next: t.fetcher.buf.cid,
			Begin: Now()}
		session.Set("ID",fmt.Sprintf("%d",nid))
		log.Printf("Created with session with id: %d", nid)
		t.fetcher.sessions[nid] = s
	} else {
		s = t.fetcher.sessions[id]
	}
	return s
}

//Method to be exposed for message retrieval. Cursor information is stored in the users GOTOJS session.
//Clients may frequently call this method to retrieve new messages.
func (t *Stream) Next(session *Session) (ret []Message) {
	if t.Config.LazyStart && !t.fetcher.running {
		log.Printf("Starting stream (lazy start).")
		t.Start()
	}

	s := t.session(session)
	s.LastAccess = Now()
	fr := NewFetchRequest(s)
	fr.max = ID(t.Config.MaxRecordCount)
	ret = t.fetcher.Fetch(fr)
	if ret != nil {
		s.Next = ret[len(ret) -1].Id + 1 // Update id
	}
	return
}

//Reset resets the users session. The cursor will be deleted and reinitialized.
func (t* Stream) Reset(session *Session) {
	s := t.session(session)
	delete(t.fetcher.sessions,s.Id)
}

// Stop stops a stream and purges all open sessions.
func (t *Stream) Stop() {
	t.fetcher.Stop()
	t.fetcher.cleanup(0) //0 indicates to cleanup all active sessions.
}

// Start checks whether the fetcher is currently running. If yes it returns immediately.
// If not it is started. This mechanism allows to stop twitter source stream if no scubscribers
// or sessions are currently open.
func (t* Stream) Start() {
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
func NewStream(source Source) (t* Stream,err error) {
	fetcher,err := NewFetcher(source) // just pass the source connection here.

	t = &Stream{	fetcher: fetcher,
			Config: Configuration{	SessionTimeout: Timestamp(DefaultSessionTimeout.Nanoseconds() / 1000),
						MaxRecordCount: DefaultMaxRecordCount,
						LazyStart: DefaultLazyStart,
						BufferBitSize: DefaultBufferSize },
			}
	filename := "streamconfig.json"
	configFile, oerr := os.Open(filename)
	if (oerr == nil) {
		log.Printf("Loading configuration from %s",filename)
		decoder := json.NewDecoder(configFile)
		decoder.Decode(t.Config)
	} else {
		log.Printf("Could not load configiguration from %s",filename)
	}

	// Start fetcher process if no lazy start is configured
	if !t.Config.LazyStart {
		log.Printf("Starting buffer routine.")
		t.Start()
	}
	return
}

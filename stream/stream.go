// Package stream of GOTOJS offers an interface to expose event or message streams.
// Stream implementations just need to implement the Source interface and define a Message type
// which is encodable as JSON.
package stream

import (
	. "github.com/sebkl/gotojs"
	"log"
	"fmt"
	"time"
	"math/rand"
	"strconv"
	"os"
	"encoding/json"
)

const (
	DefaultSessionTimeout = 30 //in seconds
	DefaultMaxRecordCount = 10000 //in count of messages
)

type Configuration struct {
	SessionTimeout int
	MaxRecordCount int
	BufferBitSize int
}

type Stream struct {
	sessions map[ID]*StreamSession
	fetcher *Fetcher
	config Configuration
}

//Internally used function to identifiy the users buffer cursor (called Stream) based on its GOTOJS session infromation.
func (t *Stream) session(session *Session) (s *StreamSession) {
	iid,_ := strconv.ParseInt(session.Get("ID"),10,64)
	id := ID(iid)

	if _,ok := t.sessions[id]; !ok {
		nid := ID(rand.Int63())
		s = &StreamSession{
			Id: nid,
			Next: t.fetcher.buf.cid,
			Begin: Timestamp(time.Now().UnixNano() / 1000)}
		session.Set("ID",fmt.Sprintf("%d",fmt.Sprintf("%d",nid)))
		log.Printf("Created with session with id: %d", nid)
		t.sessions[id] = s
	} else {
		s = t.sessions[id]
	}
	return s
}


//Method to be exposed for message retrieval. Cursor information is stored in the users GOTOJS session.
//Clients may frequently call this method to retrieve new messages.
func (t *Stream) Next(session *Session) (ret []Message) {
	t.lazyStart()
	s := t.session(session)
	fr := NewFetchRequest(s)
	fr.max = ID(t.config.MaxRecordCount)
	ret = t.fetcher.Fetch(fr)

	s.Next = ret[len(ret) -1].Id + 1 // Update id
	s.LastAccess = Timestamp(time.Now().UnixNano() / 1000)

	return
}

//Reset resets the users session. The cursor will be deleted and reinitialized.
func (t* Stream) Reset(session *Session) {
	s := t.session(session)
	delete(t.sessions,s.Id)
}

// lazyStart checks whether the fetcher is currently running. If yes it returns immediately.
// If not it is started. This mechanism allows to stop twitter source stream if no scubscribers
// or sessions are currently open.
func (t* Stream) lazyStart() {
	if t.fetcher.running {
		return
	} else {
		go func() {
			log.Printf("Starting twitterstream process.")
			if err := t.fetcher.Start(); err != nil {
				log.Fatal(err)
			}
		}()

	}
}


//Internally used function to determine whether the source stream can be stopped since to more client stream are active.
func (t* Stream) cleanup() {
	log.Printf("Performing cleanup")
	now := Timestamp(time.Now().UnixNano() / 1000)
	for k,v := range t.sessions {
		dts := Timestamp((now - v.LastAccess) / 1000000)
		if dts > Timestamp(int64(t.config.SessionTimeout)) {
			log.Printf("Killing session: %d, Timedout since %d seconds.",v.Id,dts)
			delete(t.sessions,k)
		}
	}

	if len(t.sessions) < 1 {
		t.fetcher.Stop()
	}
	log.Printf("Cleanup done.")
}

//Internally used process loop to frequently perform a cleanup run.
func (f* Stream) cleanupRunner() {
	for ;; {
		time.Sleep(10 * time.Second)
		f.cleanup()
		log.Printf("Buffer[size: %d, cid: %d, sessions: %d]",f.fetcher.buf.size,f.fetcher.buf.cid,len(f.sessions))
	}
	log.Printf("CleanupRunner stopped")
}


//NewStream creates a new Stream based on the given Source implementation.
//The default configuration can be overwritten by a configuration file named "streamconfig.json"
func NewStream(source Source) (t* Stream,err error) {
	fetcher,err := NewFetcher(source) // just pass the source connection here.

	t = &Stream{	fetcher: fetcher,
			config: Configuration{	SessionTimeout: DefaultSessionTimeout,
						MaxRecordCount: DefaultMaxRecordCount,
						BufferBitSize: DefaultBufferSize },
			sessions: make(map[ID]*StreamSession)}
	filename := "streamconfig.json"
	configFile, oerr := os.Open(filename)
	if (oerr == nil) {
		log.Printf("Loading configuration from %s",filename)
		decoder := json.NewDecoder(configFile)
		decoder.Decode(t.config)
	} else {
		log.Printf("Could not load configiguration from %s",filename)
	}

	// Start the cleanup process once.
	go func() {
		log.Printf("Starting cleanup process.")
		t.cleanupRunner()
	}()
	return
}

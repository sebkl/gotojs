//Package twitter of GOTOJS/stream offers a stream implemenation for Twitter.
//Currently it is based on the twitter location API which needs to be generalized.
//
//The client key/secret configuration for the actual twitter API can be made in a localfile
// named "twitter_account.json"
//
// A sample file is provided with the source: "twitter_account.json.sample"

package twitter

import (
	. "github.com/sebkl/gotojs/stream"
	"github.com/sebkl/twitterstream"
	"log"
	"os"
	"encoding/json"
	"errors"
)

type twitterConfiguration struct {
	APIKey string
	APISecret string
	AccessToken string
	AccessSecret string
}

type Tweet  struct {
	Long float64 `json:"long"`
	Lat float64 `json:"lat"`
	Text string `json:"text"`
	Sender string `json:"sender"`
}

type TwitterSource struct {
	config *twitterConfiguration
	conn *twitterstream.Connection
	client *twitterstream.Client
	configFile *os.File
}


//NewTwitterSource creates a new stream source based on the given configuration file.
func NewTwitterSource(filename string) (ret *TwitterSource,err error) {
	//Read twitter config from json file.
	configFile, err := os.Open(filename)
	decoder := json.NewDecoder(configFile)
	config := &twitterConfiguration{}
	decoder.Decode(config)
	client := twitterstream.NewClient(config.APIKey,config.APISecret,config.AccessToken,config.AccessSecret)

	ret = &TwitterSource{
		config: config,
		configFile: configFile,
		client: client }
	return
}

//Next fetches the nex message from the source stream.
func (s *TwitterSource) Next() (mes Message,err error) {
	if s.conn == nil {
		return mes, errors.New("Connection s not established.")
	}

	if tweet,err := s.conn.Next(); err == nil {
		var payload *Tweet

		if (tweet.Coordinates != nil) {
			payload = &Tweet{
				Long: float64(tweet.Coordinates.Long),
				Lat: float64(tweet.Coordinates.Lat),
				Text: tweet.Text,
				Sender: tweet.User.ScreenName }
		} else if (tweet.Place != nil && tweet.Place != nil) {
			payload = &Tweet{
				Long: float64(tweet.Place.BoundingBox.Points[0].Long) ,
				Lat: float64(tweet.Place.BoundingBox.Points[0].Lat),
				Text: tweet.Text,
				Sender: tweet.User.ScreenName }
		} else {
			return mes, errors.New("Invalid tweet.")
		}
		mes = NewMessage(payload)
	}
	return
}

//Close interruptes the source stream. The connection to the twitter API is closed.
func (s *TwitterSource) Close() {
	s.conn.Close()
}

//Start starts the source stream. A connection to the twitter API is established.
func (s *TwitterSource) Start() (err error) {
	log.Println("Starting twitter stream.")
	s.conn,err = s.client.Locations(twitterstream.Point{-90.0,-180.0}, twitterstream.Point{90.0,180.0})
	if err != nil {
		log.Print(err)
	}
	return
}


//NewTwitterStream create a new GOTOJS stream implementation the is bound to the twitter API stream using
// the default "twitter_account.json" configuration file.
func NewTwitterStream() (stream *Stream, err error) {
	tweetSource,err := NewTwitterSource("twitter_account.json")
	if err != nil {
		return
	}
	stream,err = NewStream(tweetSource)
	return
}

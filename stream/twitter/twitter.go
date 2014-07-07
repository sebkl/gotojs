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
	"github.com/sebkl/imgurl"
	"log"
	"os"
	"encoding/json"
	"errors"
)


const (
	THUMBNAIL_SIZE = 100
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
	Retweet bool `json:"retweet"`
	Images []string `json:"images"`
	Thumbnail string `json:"thumbnail"`
}

type TwitterSource struct {
	config *twitterConfiguration
	conn *twitterstream.Connection
	client *twitterstream.Client
	configFile *os.File
	transcoder *imgurl.TranscodeService
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
		transcoder: imgurl.NewTranscodeService(5),
		client: client }
	return
}

//Next fetches the nex message from the source stream.
func (s *TwitterSource) Next() (mes Message,err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Println("Recovered in Next", r)
			err = errors.New("Panic in Next call.")
		}
	}()

	if s.conn == nil {
		return mes, errors.New("Connection s not established.")
	}

	for ;err == nil; {
		//Take from transcoder queue
		for ;len(s.transcoder.Out) > 0; {
			res := <-s.transcoder.Out
			if tweet,ok := res.Payload.(*Tweet); ok {
				//TODO: check other images too
				tweet.Thumbnail = res.Image
				mes = NewMessage(res.Payload)
				return
			} else {
				log.Printf("Invalid payload in transcoding eesponse. Ignoring.")
			}
		}

		//Fetch from Twitter stream
		if  tweet,err := s.conn.Next(); err == nil {
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
				//return mes, err = errors.New("Invalid tweet.")
				log.Printf("Invlid tweet received. Ignoring.")
				continue
			}

			payload.Retweet = tweet.RetweetedStatus != nil

			//Check if some images are attached as Entities to the tweet.
			if len(tweet.Entities.Media) > 0 {
				iu := make([]string,len(tweet.Entities.Media))
				c := 0;
				for _,v := range tweet.Entities.Media {
					if v.Type == "photo" {
						iu[c] = v.MediaUrl
						c++
						//log.Println(v.MediaUrl)
					}
				}
				payload.Images = iu[:c]
				//TODO: change imgurl o be capable of transcoding multiple images per request.
				s.transcoder.In <- &imgurl.Request{Url: iu[0],Payload: payload,Maxheight: THUMBNAIL_SIZE, Maxwidth: THUMBNAIL_SIZE}
			} else {
				mes = NewMessage(payload)
				return mes, err
			}
		}
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

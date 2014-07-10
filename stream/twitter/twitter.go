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
	"bytes"
	"fmt"
	"net/url"
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
	IsNude bool `json:"isnude"`
	IsSensitive bool `json:"issensitive"`
}

type TwitterStreamConfig struct {
	ThumbnailMethod string `json:"ThumbnailMethod"` // [api,url,dataurl]
	ThumbnailSize int `json:"ThumbnailSize"`
	ThumbnailAPICall string `json:"ThumbnailAPICall"`
	NudeFilter bool `json:"NudeFilter"`
	TranscodeWorker int `json:"TranscodeWorker"`
	TranscodeBuffer int `json:"TranscodeBuffer"`
	BaseUrl string `json:"BaseUrl"`
}

func (s *TwitterStreamConfig) JSON() string {
	buf := new(bytes.Buffer)
	encoder := json.NewEncoder(buf)
	encoder.Encode(s)
	return buf.String()
}

func (s *TwitterStreamConfig) APIUrl(u string) string {
	return fmt.Sprintf("%s%s",s.BaseUrl,fmt.Sprintf(s.ThumbnailAPICall,url.QueryEscape(u),s.ThumbnailSize,s.ThumbnailSize))
}

type TwitterSource struct {
	config *twitterConfiguration
	streamConfig *TwitterStreamConfig
	conn *twitterstream.Connection
	client *twitterstream.Client
	configFile *os.File
	streamFile *os.File
	transcoder *imgurl.TranscodeService
	baseUrl string
}

//NewTwitterSource creates a new stream source based on the given configuration file.
func NewTwitterSource(account,stream,baseUrl string) (ret *TwitterSource,err error) {
	//Read twitter config from json file.
	configFile, err := os.Open(account)
	if err != nil {
		return
	}

	decoder := json.NewDecoder(configFile)
	config := &twitterConfiguration{}
	decoder.Decode(config)

	//Read stream config from json file.
	streamConfig := &TwitterStreamConfig{
		ThumbnailMethod: "dataurl",
		ThumbnailSize: 100,
		NudeFilter: false,
		TranscodeWorker: 5,
		TranscodeBuffer: 10,
		ThumbnailAPICall: "/Image/Thumbnail?p=%s&p=%d&p=%d",
		BaseUrl:baseUrl }
	streamFile, err := os.Open(stream)
	// If config cannot be read use default values
	if err == nil {
		decoder = json.NewDecoder(streamFile)
		decoder.Decode(streamConfig)
		err = nil
		log.Printf("Twitterstream configuration: %s",streamConfig.JSON())
	}

	client := twitterstream.NewClient(config.APIKey,config.APISecret,config.AccessToken,config.AccessSecret)

	ret = &TwitterSource{
		config: config,
		configFile: configFile,
		streamFile: streamFile,
		streamConfig: streamConfig,
		transcoder: imgurl.NewTranscodeService(streamConfig.TranscodeWorker,streamConfig.TranscodeBuffer),
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
		for ;s.transcoder.Ready(); {
			res := s.transcoder.Get()
			if tweet,ok := res.Payload.(*Tweet); ok {
				//TODO: check other images too
				tweet.Thumbnail = res.Image
				if res.Tags != nil && len(res.Tags) > 0 {
					if in,ok := res.Tags[0].(bool);ok {
						tweet.IsNude = in
					}
				}
				mes = NewMessage(tweet)
				return
			} else {
				log.Printf("Invalid payload in transcoding Response. Ignoring.")
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
					IsSensitive: *tweet.PossiblySensitive,
					Sender: tweet.User.ScreenName }
			} else if (tweet.Place != nil && tweet.Place != nil) {
				payload = &Tweet{
					Long: float64(tweet.Place.BoundingBox.Points[0].Long) ,
					Lat: float64(tweet.Place.BoundingBox.Points[0].Lat),
					Text: tweet.Text,
					IsSensitive: *tweet.PossiblySensitive,
					Sender: tweet.User.ScreenName }
			} else {
				//return mes, err = errors.New("Invalid tweet.")
				log.Printf("Invlid tweet received. Ignoring.")
				continue
			}

			payload.Retweet = tweet.RetweetedStatus != nil

			//Check if some images are attached as Entities to the tweet.
			//If transcoding buffer is full, this image will ignored.
			if len(tweet.Entities.Media) > 0 && !s.transcoder.Full(){
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

				var filter []imgurl.Filter
				if s.streamConfig.NudeFilter {
					filter = []imgurl.Filter{imgurl.NudeFilter}
				}

				switch s.streamConfig.ThumbnailMethod {
					case "api":
						payload.Thumbnail = s.streamConfig.APIUrl(iu[0])
					case "url":
						payload.Thumbnail = iu[0]
					default:
						req := &imgurl.Request{
							Url: iu[0],
							Payload: payload,
							Filters: filter,
							Maxheight: s.streamConfig.ThumbnailSize,
							Maxwidth: s.streamConfig.ThumbnailSize }

						s.transcoder.Push(req)
						continue
				}
			}

			return NewMessage(payload), err
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
func NewTwitterStream(baseUrl string) (stream *Stream, err error) {
	tweetSource,err := NewTwitterSource("twitter_account.json","twitter_stream.json",baseUrl)
	if err != nil {
		return
	}

	stream,err = NewStream(tweetSource)
	return
}

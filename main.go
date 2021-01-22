package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	runtime "github.com/aws/aws-lambda-go/lambda"
	"github.com/hekmon/plexwebhooks"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type EpisodeData struct {
	TvId    string
	Season  int
	Episode int
}

func HandleRequest(req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {

	_, params, err := mime.ParseMediaType(req.Headers["Content-Type"])
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       err.Error(),
		}, nil
	}

	reader := multipart.NewReader(strings.NewReader(req.Body), params["boundary"])


	payload, _, err := plexwebhooks.Extract(reader)

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       err.Error(),
		}, nil
	}



	tvtimeAccessToken := req.QueryStringParameters["accessToken"]
	user := req.QueryStringParameters["username"]


	fmt.Printf("Event : %+v\n", payload.Event)
	if payload.Event != "media.scrobble" {
		return events.APIGatewayProxyResponse{
			StatusCode: 202,
			Body:       "{'message': 'BadEvent'}",
		}, nil
	}
	fmt.Printf("Account : %+v\n", payload.Account.Title)
	if payload.Account.Title != user {
		return events.APIGatewayProxyResponse{
			StatusCode: 202,
			Body:       "{'message': 'BadUser'}",
		}, nil
	}
	fmt.Printf("Content type : %+v\n", payload.Metadata.Type)
	if payload.Metadata.Type != "episode" {
		return events.APIGatewayProxyResponse{
			StatusCode: 202,
			Body:       "{'message': 'BadType'}",
		}, nil
	}

	episodeData, err := parseShow(payload.Metadata.GUID.String())
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	fmt.Printf("Parsed show : %+v\n", episodeData)
	err = markAShowAsWatched(episodeData, tvtimeAccessToken)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	fmt.Printf("Episode is marked as watched :")
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       "The show has benn marked as watched",
	}, nil
}

func main() {
	runtime.Start(HandleRequest)
}

type TVTimeResponse struct {
	Result string `json:"result"`
}

func parseShow(stringToParse string) (EpisodeData, error) {
	r := regexp.MustCompile(`://([0-9]+)/([0-9]+)/([0-9]+)`)
	data := r.FindStringSubmatch(stringToParse)

	season, err := strconv.ParseInt(data[2], 10, 64)
	if err != nil {
		return EpisodeData{}, err
	}
	episode, err := strconv.ParseInt(data[3], 10, 64)
	if err != nil {
		return EpisodeData{}, err
	}

	return EpisodeData{
		TvId:    data[1],
		Season:  int(season),
		Episode: int(episode),
	}, nil
}

func markAShowAsWatched(episode EpisodeData, accessToken string) error {
	data := url.Values{
		"access_token": {accessToken},
		"show_id":      {episode.TvId},
		"season":       {strconv.Itoa(episode.Season)},
		"episode":      {strconv.Itoa(episode.Episode)},
	}
	response, err := http.PostForm("https://api.tvtime.com/v1/show_progress", data)
	if err != nil {
		return err
	}

	var res TVTimeResponse

	json.NewDecoder(response.Body).Decode(&res)

	fmt.Printf("Error in TVTIme Request : %+v\n", err)
	fmt.Printf("TVTime response: %+v\n", res)

	if res.Result != "OK" {
		return errors.New("response not ok")
	}
	return nil
}

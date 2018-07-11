package redis

import (
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	"github.com/fission/fission/redis/build/gen"
	"strings"
	"net/url"
)

func NewClient() redis.Conn {
	// TODO: Load redis ClusterIP from environment variable / configmap
	c, err := redis.Dial("tcp", "10.102.223.159:6379")
	if err != nil {
		log.Fatalf("Could not connect: %v\n", err)
	}
	return c
}

func EndRecord(triggerName string, recorderName string, reqUID string, request *http.Request, originalUrl url.URL, response *http.Response, namespace string, timestamp int64) {
	// Case where the function should not have been recorded
	if len(reqUID) == 0 {
		return
	}

	replayed := originalUrl.Query().Get("replayed")

	log.Info("EndRecord: URL > ", originalUrl.String(), " with queries ", replayed)

	if replayed == "true" {
		log.Info("This was a replayed request.")
		return
	}

	client := NewClient()

	url := make(map[string]string)
	url["Host"] = request.URL.Host
	url["Path"] = originalUrl.String()	// Previously request.URL.Path
	header := make(map[string]string)
	for key, value := range request.Header {
		header[key] = strings.Join(value, ",")
	}

	form := make(map[string]string)
	for key, value := range request.Form {
		form[key] = strings.Join(value, ",")
	}

	postForm := make(map[string]string)
	for key, value := range request.PostForm {
		postForm[key] = strings.Join(value, ",")
	}

	req := &redisCache.Request{
		Method:   "GET",
		URL:      url,
		Header:   header,
		Host:     request.Host,
		Form:     form,
		PostForm: postForm,
	}

	resp := &redisCache.Response{
		Status: response.Status,
		StatusCode: int32(response.StatusCode),
	}

	ureq := &redisCache.UniqueRequest {
		Req: req,
		Resp: resp,
		Trigger: triggerName,			// TODO: Why is this here when Trigger is set as a separate field?
	}

	data, err := proto.Marshal(ureq)
	if err != nil {
		log.Fatal("Marshalling UniqueRequest error: ", err)
	}

	_, err = client.Do("HMSET", reqUID, "ReqResponse", data, "Timestamp", timestamp, "Trigger", triggerName)
	if err != nil {
		panic(err)
	}

	_, err = client.Do("LPUSH", recorderName, reqUID)
	if err != nil {
		panic(err)
	}
}
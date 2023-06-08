package isquestionclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
)

const (
	iqQuestionEndPoint = "http://172.28.166.154:8080/is_question"
)

type IsQuestionJob struct {
	input  string
	result chan bool
}

type IsQuestionContext struct {
	workers            int
	isQuestionJobQueue chan IsQuestionJob
	stop               chan bool
	mutex              sync.Mutex
}

func Initialize(workers int) (*IsQuestionContext, error) {
	if workers <= 0 {
		workers = 1
	}
	client := &IsQuestionContext{
		workers:            workers,
		isQuestionJobQueue: make(chan IsQuestionJob, workers),
		stop:               make(chan bool, 1),
	}

	go client.processIsQuestionJobs()

	return client, nil
}

func (client *IsQuestionContext) processIsQuestionJobs() {
	for {
		select {
		case job := <-client.isQuestionJobQueue:
			client.mutex.Lock()
			isQuestion, err := client.callIsQuestionAPI(job.input)
			client.mutex.Unlock()
			if err != nil {
				log.Printf("Failed to check if %s is a question: %v", job.input, err)
				job.result <- false
				continue
			}
			job.result <- isQuestion
		case <-client.stop:
			return
		}
	}
}

func (client *IsQuestionContext) callIsQuestionAPI(input string) (bool, error) {
	jsonData := map[string]string{
		"sentence": input,
	}
	jsonValue, _ := json.Marshal(jsonData)

	response, err := http.Post(iqQuestionEndPoint, "application/json", bytes.NewBuffer(jsonValue))

	if err != nil {
		log.Printf("The HTTP request failed with error %s\n", err)
		return false, err
	}

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return false, err
	}

	var result map[string]bool
	err = json.Unmarshal([]byte(data), &result)
	if err != nil {
		return false, err
	}

	return result["is_question"], nil
}

func (client *IsQuestionContext) IsQuestion(input string) (bool, error) {
	result := make(chan bool)
	client.isQuestionJobQueue <- IsQuestionJob{input: input, result: result}
	return <-result, nil
}

func (client *IsQuestionContext) Close() {
	close(client.stop)
	close(client.isQuestionJobQueue)
}

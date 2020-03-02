package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	"github.com/bndr/gojenkins"
)

func jenkinsJob(args *Args) (url string, err error) {
	baseurl := fmt.Sprintf("https://%s/", args.JenkinsHost)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	jenkins := gojenkins.CreateJenkins(client, baseurl,
		args.User, args.JenkinsToken)

	job, err := jenkins.GetJob(args.JenkinsJob)

	if err != nil {
		fmt.Printf("error getting job: %s\n", err)
		return
	}

	params := map[string]string{
		"PVT_BRANCH_NAME": args.Branch,
		"CICD_KEYPAIR":    args.User,
		"SUITE_TO_RUN":    args.JenkinsSuite,
	}
	id, err := job.InvokeSimple(params)
	if err != nil {
		fmt.Printf("error starting job: %s\n", err)
		return
	}

	task, err := jenkins.GetQueueItem(id)
	for err == nil {
		if task.Raw.Executable.Number != 0 {
			break
		}
		time.Sleep(time.Second)
		task, err = jenkins.GetQueueItem(id)
	}
	url = task.Raw.Executable.URL
	fmt.Printf("Jenkins job started: %s\n", url)
	return
}

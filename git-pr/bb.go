package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"gotools/rest"
)

type BbUser struct {
	Id   string `json:"username,omitempty"`
	Name string `json:"display_name,omitempty"`
}

type PullRequestBody struct {
	Source struct {
		Branch struct {
			Name string `json:"name,omitempty"`
		} `json:"branch,omitempty"`
	} `json:"source,omitempty"`
	Destination struct {
		Branch struct {
			Name string `json:"name,omitempty"`
		} `json:"branch,omitempty"`
	} `json:"destination,omitempty"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Message     string   `json:"message"`
	Reviewers   []BbUser `json:"reviewers"`
	Close       bool     `json:"close_source_branch"`
}

type PullRequest struct {
	Id    int `json:"id,omitempty"`
	Links struct {
		Html struct {
			Href string `json:"href,omitempty"`
		} `json:"html,omitempty"`
	} `json:"links,omitempty"`
}

type PullRequestMerge struct {
	Strategy string `json:"merge_strategy"`
}

func (b *Bb) members(r *rest.Rest, args *Args) (users []User) {

	if args.Team == "" {
		return
	}

	resp, err := r.Get(fmt.Sprintf("/teams/%s/members", args.Team), nil, nil)
	if err != nil {
		log.Panic(err)
	}

	bbusers := []BbUser{}
	unpack(resp, &bbusers)

	for _, u := range bbusers {
		if u.Id != args.User {
			users = append(users, User{Id: u.Id, Name: u.Name})
		}
	}

	return

}

func (b *Bb) create() {

	args := b.args
	r := rest.NewRest("https://api.bitbucket.org/2.0",
		args.User, args.Password, args.Verbose)

	users := b.members(r, args)

	fn := prepare(args, users)
	defer os.Remove(fn)

	for {
		subj, desc := edit(fn)
		meta, desc := trailers(desc)
		users := reviewers(meta)
		if strings.HasPrefix(subj, "!") {
			return
		}

		body := PullRequestBody{
			Title:       subj,
			Description: desc,
		}
		for _, u := range users {
			body.Reviewers = append(body.Reviewers, BbUser{Id: u.Id, Name: u.Name})
		}
		body.Source.Branch.Name = args.Branch
		body.Destination.Branch.Name = args.Upstream

		path := fmt.Sprintf("/repositories/%s/%s/pullrequests", args.Owner, args.Repo)
		res, err := r.Post(path, body)
		if err != nil {
			log.Print(err)
			continue
		}

		prr := PullRequest{}
		unpack(res, &prr)
		dump("Pull request", prr)
		break
	}

}

func (b *Bb) merge() {
	args := b.args

	rest := rest.NewRest("https://api.bitbucket.org/2.0",
		args.User, args.Password, args.Verbose)

	query := fmt.Sprintf("state=\"OPEN\" AND author.username=\"%s\" AND source.branch.name=\"%s\"",
		args.User, args.Branch)
	resp, err := rest.Get(fmt.Sprintf("/repositories/%s/%s/pullrequests", args.Owner, args.Repo),
		url.Values{
			"q": []string{query},
		}, nil)

	if err != nil {
		log.Panic(err)
	}
	prs := []PullRequest{}
	unpack(resp, &prs)
	dump("Resp:", prs)

	m := PullRequestMerge{Strategy: "merge_commit"}
	for _, pr := range prs {
		url := fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/merge",
			args.Owner, args.Repo, pr.Id)
		rest.Post(url, m)
	}
}

func (b *Bb) test() {
}

type Bb struct {
	args *Args
}

func bb(args *Args) Git {
	return &Bb{
		args: args,
	}
}

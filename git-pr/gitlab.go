package main

import (
	"fmt"
	"gotools/rest"
	"log"
	"net/url"
	"os"
	"strings"
)

type Gitlab struct {
	args Args
	r    *rest.Rest
	url  string
}

func NewGitlab(args Args) Git {
	g := Gitlab{}
	g.args = args
	g.url = "https://git.eng.vmware.com/api/v4/"
	g.r = rest.NewRest(g.url,
		args.User, args.Password, args.Verbose)
	return &g
}

func (g *Gitlab) request(method, path string, data interface{}) ([]map[string]interface{}, error) {
	x, err := g.r.Do(method, g.url+path, nil, data)
	return x, err
}

func (g *Gitlab) Get(path string) []map[string]interface{} {
	x, err := g.request("GET", path, nil)
	if err != nil {
		log.Panic(err)
	}
	return x
}

func (g *Gitlab) Post(path string, data interface{}) ([]map[string]interface{}, error) {
	return g.request("POST", path, data)
}

func (g *Gitlab) Put(path string, data interface{}) ([]map[string]interface{}, error) {
	return g.request("PUT", path, data)
}

func (g *Gitlab) test() {
	x := g.Get(g.args.args[0])
	dump("result", x)
}

type GitlabUser struct {
	Uid  int    `json:"id"`
	Id   string `json:"username"`
	Name string `json:"name"`
}

func (g *Gitlab) members() (users []GitlabUser) {
	args := g.args

	x := g.Get(fmt.Sprintf("/groups/%s/members", url.QueryEscape(args.Team)))
	unpack(x, &users)

	return users
}

type GitlabMergeRequest struct {
	Id     string `json:"id"`
	Src    string `json:"source_branch"`
	Dst    string `json:"target_branch"`
	Title  string `json:"title"`
	Descr  string `json:"description"`
	Labels string `json:"labels"`
}
type GitlabMergeApprovers struct {
	Id     int   `json:"id"`
	Iid    int   `json:"iid"`
	Users  []int `json:"approver_ids",omitempty`
	Groups []int `json:"approver_group_ids",omitempty`
}

func (g *Gitlab) submit(subj, desc string, ids []int) error {

	args := g.args
	proj := url.QueryEscape(args.Owner + "/" + args.Repo)
	mr := GitlabMergeRequest{
		Id:     proj,
		Src:    args.Branch,
		Dst:    args.Upstream,
		Title:  subj,
		Descr:  desc,
		Labels: args.Label,
	}
	path := fmt.Sprintf("projects/%s/merge_requests", proj)

	resp, err := g.Post(path, &mr)
	if err != nil {
		log.Panic(err)
	}

	mri := struct {
		Id        int    `json:"id"`
		Iid       int    `json:"iid"`
		ProjectId int    `json:"project_id"`
		Url       string `json:"web_url"`
	}{}
	unpack(resp[0], &mri)
	dump("mr:", &mri)
	mra := GitlabMergeApprovers{
		Id:     mri.Id,
		Iid:    mri.Iid,
		Users:  ids,
		Groups: []int{},
	}

	path = fmt.Sprintf("projects/%d/merge_requests/%d/approvers",
		mri.ProjectId, mri.Iid)

	resp, err = g.Put(path, &mra)
	if err != nil {
		log.Panic(err)
	}

	return err
}

func (g *Gitlab) create() {
	members := g.members()
	users := []User{}
	for _, u := range members {
		if u.Id != g.args.User {
			users = append(users, User{Id: u.Id, Name: u.Name})
		}
	}

	fn := prepare(g.args, users)
	defer os.Remove(fn)

	for {
		subj, desc := edit(fn)
		if strings.HasPrefix(subj, "!") {
			return
		}
		users, desc = reviewers(desc)

		ids := []int{}
		for _, u := range users {
			for _, m := range members {
				if u.Id == m.Id {
					ids = append(ids, m.Uid)
					break
				}
			}
		}
		if g.submit(subj, desc, ids) == nil {
			break
		}
	}
}

func (g *Gitlab) merge() {

}

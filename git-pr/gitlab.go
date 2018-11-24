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

func (g *Gitlab) test() {
	x := g.Get(g.args.args[0])
	dump("result", x)
}

type GitlabUser struct {
	//Id    int    `json:"id"`
	Id   string `json:"username"`
	Name string `json:"name"`
}

func (g *Gitlab) members() (users []User) {
	args := g.args

	x := g.Get(fmt.Sprintf("/groups/%s/members", url.PathEscape(args.Team)))
	m := []GitlabUser{}
	unpack(x, &m)

	for _, u := range m {
		//if u.Id != args.User {
		users = append(users, User{Id: u.Id, Name: u.Name})
		//}
	}

	return users
}

type GitlabMergeRequest struct {
	Id     string   `json:"id"`
	Src    string   `json:"source_branch"`
	Dst    string   `json:"target_branch"`
	Title  string   `json:"title"`
	Descr  string   `json:"description"`
	Users  []string `json:"approver_ids"`
	Groups []string `json:"approver_group_ids"`
}

func (g *Gitlab) submit(subj, desc string, m []User) error {

	args := g.args
	proj := args.Owner + "%2F" + args.Repo
	mr := GitlabMergeRequest{
		Id:     proj,
		Src:    args.Branch,
		Dst:    args.Upstream,
		Title:  subj,
		Descr:  desc,
		Groups: []string{args.Team},
	}
	for _, u := range m {
		mr.Users = append(mr.Users, u.Id)
	}
	path := fmt.Sprintf("projects/%s/merge_requests", proj)
	_, err := g.Post(path, &mr)
	return err
}

func (g *Gitlab) create() {
	users := g.members()

	fn := prepare(g.args, users)
	defer os.Remove(fn)

	for {
		subj, desc := edit(fn)
		if strings.HasPrefix(subj, "!") {
			return
		}
		users, desc = reviewers(desc)

		if g.submit(subj, desc, users) == nil {
			break
		}
	}
}

func (g *Gitlab) merge() {

}

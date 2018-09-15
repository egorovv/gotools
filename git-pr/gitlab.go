package main

import (
	"fmt"
	"gotools/rest"
	"log"

	gitlab "github.com/xanzy/go-gitlab"
)

func test_gitlab(args Args) {
	git := gitlab.NewClient(nil, "https://git.eng.vmware.com", args.Password)
	//projects, _, err := git.Projects.ListProjects(nil)
	//lo := gitlab.ListGroupsOptions{}
	//memb, _, err := git.Groups.ListGroups(&lo)
	memb, _, err := git.GroupMembers.ListGroupMembers(args.Team, nil)
	if err != nil {
		log.Panic(err)
	}
	dump("members", memb)

}

func gitlab_req(args Args, req string) {
	url := "https://git.eng.vmware.com/api/v4"
	r := rest.NewRest(url,
		args.User, args.Password, args.Verbose)

	u := fmt.Sprintf("%s/%s", url, req)
	x, err := r.Do("GET", u, nil, nil)
	if err != nil {
		log.Panic(err)
	}
	dump("memb", x)
}

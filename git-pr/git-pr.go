package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"gotools/util"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

type Args struct {
	Owner        string `json:"owner,omitempty"`
	Repo         string `json:"repo,omitempty"`
	User         string `json:"user,omitempty"`
	Password     string `json:"password,omitempty"`
	Branch       string `json:"branch,omitempty"`
	Upstream     string `json:"upstream,omitempty"`
	Team         string `json:"team,omitempty"`
	Label        string `json:"label,omitempty"`
	Remove       bool   `json:"remove,omitempty"`
	Verbose      bool   `json:"verbose"`
	JenkinsHost  string `json:"jenkins-host"`
	JenkinsToken string `json:"jenkins-token"`
	JenkinsJob   string `json:"jenkins-job"`
	JenkinsSuite string `json:"jenkins-suite"`
	JenkinsKey   string `json:"jenkins-key"`
	remote       string
	args         []string
}

type User struct {
	Name string
	Id   string
}

type Git interface {
	create()
	jenkins()
	merge()
	test()
}

func dump(prefix string, v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")

	fmt.Printf("%s: %s\n", prefix, b)
}

func unpack(src, dst interface{}) {
	b, err := json.Marshal(src)
	if err != nil {
		log.Panic(err)
	}
	err = json.Unmarshal(b, dst)
	if err != nil {
		log.Panic(err)
	}
}

func strip(s string) string {
	regex, err := regexp.Compile(`(?m)^#.*$`)
	if err != nil {
		log.Panic(err)
	}
	return regex.ReplaceAllString(s, "")
}

func trailers(s string) (elts map[string][]string, desc string) {
	elts = make(map[string][]string)
	pattern := fmt.Sprintf(`(?m)^[A-Z][a-z]+-[A-Z][a-z]+: .*$`)
	regex, err := regexp.Compile(pattern)
	if err != nil {
		log.Panic(err)
	}
	lines := regex.FindAllString(s, -1)
	for _, l := range lines {
		parts := strings.SplitN(l, ":", 2)
		if len(parts) != 2 {
			continue
		}
		vals, _ := elts[parts[0]]
		vals = append(vals, strings.TrimSpace(parts[1]))
		elts[parts[0]] = vals
	}
	desc = strings.TrimSpace(regex.ReplaceAllString(s, ""))
	return
}

func trailer(meta map[string][]string, key string) (val string) {
	if len(meta[key]) > 0 {
		val = meta[key][0]
	}
	return
}

func reviewers(meta map[string][]string) (m []User) {
	users, _ := meta["Review-By"]
	for _, user := range users {
		u := User{Id: strings.Fields(user)[0]}
		m = append(m, u)
	}
	return
}

var commentBody = `
"{{.Args.JenkinsSuite}}" test suite results -

{{.Body}}

---
Brought to you by git-pr
[https://gitlab.eng.vmware.com/egorovv/gotools/tree/master/git-pr]
`

var requestBody = `#
# Edit pull request title and description, remove starting '!'.
#
# All lines starting with # will be removed. Of the remaining the first
# line will be used as a title and the rest as description.
# Subject starting with ! will cause the oeration to abort.
# You can comment/uncomment trailers at the end to control
# extra actions
#
# User: {{ .Args.User }}
# Branch: {{ .Args.Branch }}
# Upstream: {{ .Args.Upstream }}
# Owner/Repo: {{ .Args.Owner }}/{{ .Args.Repo }}
# Remove: {{ .Args.Remove }}
#
{{.Body}}

Notify @{{.Args.Team}}

####### trailers ##########
Gitlab-Label: {{ .Args.Label }}
# This PR will trigger the following test
Jenkins-Suite: {{.Args.JenkinsSuite}}

# This PR will add the following users to approvers
{{range .Members }}#Review-By: {{ .Id }} <{{ .Name }}>
{{end}}

`

func expand(args *Args, body, text string) string {
	t, err := template.New("COMMENT").Parse(body)
	if err != nil {
		log.Panic(err)
	}

	data := struct {
		Body string
		Args *Args
	}{
		Body: text,
		Args: args,
	}

	buf := bytes.NewBufferString("")
	err = t.Execute(buf, data)
	return buf.String()
}

func prepare(args *Args, m []User) (fn string) {

	text := util.Sh(`git`, `log`, `--reverse`, `@{u}..`, `--pretty= - %B`)
	text = text[2:]

	t, err := template.New("PR").Parse(requestBody)

	data := struct {
		Body    string
		Members []User
		Args    *Args
	}{
		Body:    text,
		Members: m,
		Args:    args,
	}

	f, err := ioutil.TempFile("/tmp", ".bbpr")
	if err != nil {
		log.Panic(err)
	}
	fn = f.Name()

	err = t.Execute(f, data)
	f.Close()
	return
}

func edit(fn string) (subj, desc string) {
	editor, ok := os.LookupEnv("GIT_EDITOR")
	if !ok {
		editor = "/usr/bin/editor"
	}
	cmd := exec.Command(editor, fn)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	err := cmd.Run()
	if err != nil {
		log.Panic(err)
	}
	db, err := ioutil.ReadFile(fn)
	if err != nil {
		log.Panic(err)
	}

	parts := strings.SplitN(strings.TrimSpace(strip(string(db))), "\n", 2)
	subj = strings.TrimSpace(parts[0])
	desc = strings.TrimSpace(parts[1])
	return
}

func install(args Args) {
	exe := os.Args[0]
	util.Sh(`git`, `config`, `--global`, `alias.pr`, `!`+exe)
	util.SaveGitFlags("pr")
}

func git_detect(args *Args) {

	upstream := strings.SplitN(
		util.Sh(`git`, `rev-parse`, `--abbrev-ref`, `@{u}`), "/", 2)

	remote := strings.Split(
		util.Sh(`git`, `remote`, `get-url`, upstream[0]), ":")

	repo := strings.SplitN(remote[len(remote)-1], "/", 2)

	args.remote = upstream[0]
	args.Upstream = upstream[1]
	args.Owner = repo[0]
	args.Repo = strings.TrimSuffix(repo[1], ".git")
}

func main() {
	args := Args{
		Team:        "velocloud/dp",
		Branch:      "{{.Branch}}",
		JenkinsHost: "jenkins2.eng.velocloud.net",
		JenkinsJob:  "devtest-pvt-branch-validator",
	}

	git_detect(&args)

	util.GetFlags(&args, "pr")
	if len(flag.Args()) > 0 {
		args.args = flag.Args()[1:]
	}

	t, _ := template.New("pr").Parse(args.Branch)
	b := bytes.NewBufferString("")
	args.Branch = util.Sh(`git`, `symbolic-ref`, `--short`, `HEAD`)
	t.Execute(b, args)
	args.Branch = b.String()

	git := NewGitlab(&args)

	switch flag.Arg(0) {
	case "install":
		install(args)
	case "jenkins":
		util.Sh(`git`, `push`, `-f`, args.remote, fmt.Sprintf("HEAD:%s", args.Branch))
		git.jenkins()
	case "merge":
		git.merge()
	case "test":
		git.test()
	case "", "create":
		util.Sh(`git`, `push`, `-f`, args.remote, fmt.Sprintf("HEAD:%s", args.Branch))
		git.create()
	default:
		log.Panic(flag.Args())
	}

}

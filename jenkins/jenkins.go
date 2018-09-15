package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path"
	"path/filepath"

	japi "github.com/yosida95/golang-jenkins"
)

func dump(v interface{}) {

	out, _ := json.MarshalIndent(v, "", " ")
	fmt.Printf("%s\n", out)
}

type Progress struct {
	src   io.Reader
	total int
	size  int
}

func (p *Progress) Read(b []byte) (n int, err error) {
	n, err = p.src.Read(b)

	p.total += n
	fmt.Printf("%d of %d, %d%%\r", p.total, p.size, p.total*100/p.size)
	return
}

func GetArtifact(args *Args, jenkins *japi.Jenkins, build japi.Build, artifact japi.Artifact) error {
	requestUrl := fmt.Sprintf("%s/artifact/%s", build.Url, artifact.RelativePath)
	req, err := http.NewRequest("GET", requestUrl, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(args.User, args.Token)

	res, err := args.client.Do(req)
	if err != nil {
		return err
	}

	defer res.Body.Close()
	p := &Progress{
		src:  res.Body,
		size: int(res.ContentLength),
	}

	f, err := os.Create(artifact.FileName)
	if err != nil {
		return nil
	}
	defer f.Close()
	_, err = io.Copy(f, p)
	fmt.Printf("\n")
	return err
}

type Args struct {
	Cmd     string `json:"cmd"`
	Host    string `json:"host"`
	User    string `json:"user"`
	Token   string `json:"token"`
	Job     string `json:"job"`
	Files   string `json:"files"`
	Build   int    `json:"build"`
	Verbose bool   `json:"verbose"`
	client  *http.Client
}

func load(a *Args) {
	user, err := user.Current()
	if err != nil {
		return
	}
	a.User = user.Username

	fn := path.Join(user.HomeDir, ".jenkinsrc")

	if f, err := os.Open(fn); err == nil {
		defer f.Close()
		p := json.NewDecoder(f)
		p.Decode(a)
	}
}

func main() {

	args := Args{
		Host: "jenkins2.eng.velocloud.net",
		Job:  "master-nightly-build",
	}

	load(&args)

	flag.StringVar(&args.Cmd, "cmd", "get", "job name")
	flag.StringVar(&args.Job, "job", args.Job, "job name")
	flag.StringVar(&args.Files, "files", args.Files, "file pattern")
	flag.IntVar(&args.Build, "build", args.Build, "build no")
	flag.StringVar(&args.Host, "host", args.Host, "jenkins")
	flag.StringVar(&args.User, "user", args.User, "user")
	flag.StringVar(&args.Token, "token", args.Token, "token")
	flag.BoolVar(&args.Verbose, "verbose", args.Verbose, "verbose")
	flag.Parse()

	auth := &japi.Auth{
		Username: args.User,
		ApiToken: args.Token,
	}

	fmt.Printf("auth %s:%s\n", args.User, args.Token)

	baseurl := fmt.Sprintf("https://%s/", args.Host)
	jenkins := japi.NewJenkins(auth, baseurl)

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	args.client = &http.Client{Transport: tr}
	jenkins.SetHTTPClient(args.client)

	if args.Cmd == "ls" {
		if args.Verbose {
			jobs, err := jenkins.GetJobs()
			if err != nil {
				fmt.Printf("error getting jobs : %s\n", err)
				return
			}
			for _, j := range jobs {
				fmt.Printf("job %s\n", j.Name)
			}
		}
		return
	}

	j, err := jenkins.GetJob(args.Job)

	if err != nil {
		fmt.Printf("error getting job %s : %s\n", args.Job, err)
		return
	}

	if args.Cmd == "build" {
		err := jenkins.Build(j, nil)
		if err != nil {
			fmt.Printf("error getting build %d : %s\n", args.Build, err)
		}
		return
	}

	if args.Build <= 0 {
		args.Build = j.LastSuccessfulBuild.Number
	}
	b, err := jenkins.GetBuild(j, args.Build)
	if err != nil {
		fmt.Printf("error getting build %d : %s\n", args.Build, err)
		return
	}

	if args.Verbose {
		o, _ := jenkins.GetBuildConsoleOutput(b)
		fmt.Printf("\n%s\n", string(o))
	}

	a := b.Artifacts
	for _, x := range a {
		if args.Verbose {
			fmt.Printf("%s\n", x.FileName)
		}
		if m, _ := filepath.Match(args.Files, x.FileName); m {
			fmt.Printf("%s\n", x.FileName)
			GetArtifact(&args, jenkins, b, x)
		}
	}
}

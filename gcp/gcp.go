package main

import (
	"context"
	"gotools/util"
	"log"
	"net/http"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
)

type Args struct {
	Verbose      bool
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Project      string `json:"project"`
	Name         string `json:"name"`
	Zone         string `json:"zone"`
	Image        string `json:"image"`
	Network      string `json:"network"`
	Subnetwork   string `json:"subnet"`
}

func create(client *http.Client, args *Args) {
	service, err := compute.New(client)
	if err != nil {
		log.Fatalf("Unable to create Compute service: %v", err)
	}

	projectID := args.Project
	instanceName := args.Name

	prefix := "https://www.googleapis.com/compute/v1/projects/" + projectID
	imageURL := "https://www.googleapis.com/compute/v1/projects/debian-cloud/global/images/debian-7-wheezy-v20140606"
	zone := args.Zone

	// Show the current images that are available.
	res, err := service.Images.List(projectID).Do()
	for _, i := range res.Items {
		log.Printf("Instance %s", *i)
	}
	return

	instance := &compute.Instance{
		Name:        args.Name,
		Description: "cle created",
		MachineType: prefix + "/zones/" + args.Zone + "/machineTypes/n1-standard-1",
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				Type:       "PERSISTENT",
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskName:    "my-root-pd",
					SourceImage: imageURL,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "External NAT",
					},
				},
				Network: prefix + "/global/networks/default",
			},
		},
		ServiceAccounts: []*compute.ServiceAccount{
			{
				Email: "default",
				Scopes: []string{
					compute.DevstorageFullControlScope,
					compute.ComputeScope,
				},
			},
		},
	}

	op, err := service.Instances.Insert(projectID, zone, instance).Do()
	log.Printf("Got compute.Operation, err: %#v, %v", op, err)
	etag := op.Header.Get("Etag")
	log.Printf("Etag=%v", etag)

	inst, err := service.Instances.Get(projectID, zone, instanceName).IfNoneMatch(etag).Do()
	log.Printf("Got compute.Instance, err: %#v, %v", inst, err)
	if googleapi.IsNotModified(err) {
		log.Printf("Instance not modified since insert.")
	} else {
		log.Printf("Instance modified since insert.")
	}
}

func main() {
	args := Args{}

	util.GetFlags(&args, "gcp")

	if args.Verbose {
		util.Dump("args", args)
	}

	scopes := strings.Join([]string{
		compute.DevstorageFullControlScope,
		compute.ComputeScope,
	}, " ")

	config := &oauth2.Config{
		ClientID:     args.ClientID,
		ClientSecret: args.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{scopes},
	}

	ctx := context.Background()
	c := newOAuthClient(ctx, config)

	create(c, &args)
}

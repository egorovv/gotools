package main

import (
	"bytes"
	"context"
	"fmt"
	"gotools/util"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"text/template"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/dns/v1"
)

type Args struct {
	Verbose      bool   `json:"verbose"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Project      string `json:"project"`
	Region       string `json:"region"`
	Name         string `json:"name"`
	Image        string `json:"image"`
	Network      string `json:"network"`
	Subnet       string `json:"subnet"`
	Type         string `json:"type"`
	UserData     string `json:"user_data"`
	Zone         string `json:"zone"`
	SshKey       string `json:"ssh_key"`
	service      *compute.Service
	client       *http.Client
}

func wait(args *Args, op *compute.Operation, err error) {
	for err != nil {
		log.Fatalf("%s", err)
	}
	service := args.service
	log.Printf("waiting %s", op)
	_, err = service.ZoneOperations.Wait(args.Project, args.Region, op.Name).Do()

	if err != nil {
		log.Fatalf("%s", err)
	}
}

func updateDNS(args *Args, ip string) {

	d, err := dns.New(args.client)

	z, err := d.ManagedZones.Get(args.Project, args.Zone).Do()

	rs := dns.ResourceRecordSet{
		Name:    args.Name + "." + z.DnsName,
		Type:    "A",
		Ttl:     300,
		Rrdatas: []string{ip},
	}

	_, err = d.Projects.ManagedZones.Rrsets.Delete(args.Project, args.Zone, rs.Name, rs.Type).Do()
	log.Printf("deleting %s - %s", rs.Name, err)
	_, err = d.Projects.ManagedZones.Rrsets.Create(args.Project, args.Zone, &rs).Do()
	log.Printf("creating %s - %s", rs.Name, err)
}

func create(client *http.Client, args *Args) {
	service, err := compute.New(client)
	if err != nil {
		log.Fatalf("Unable to create Compute service: %v", err)
	}
	args.service = service

	inst, err := service.Instances.Get(args.Project, args.Region, args.Name).Do()
	if err == nil {
		log.Printf("Cleaning up instance %s", inst)
		op, err := service.Instances.Delete(args.Project, args.Region, args.Name).Do()
		wait(args, op, err)
	}

	user_data := ""
	if args.UserData != "" {
		str, err := ioutil.ReadFile(args.UserData)
		if err != nil {
			log.Printf("Bad user_data %s: %s", args.UserData, err)
			return
		}
		t, err := template.New("").Parse(string(str))
		var b bytes.Buffer
		err = t.Execute(&b, args)
		user_data = string(b.String())
		log.Printf("user data %s", user_data)
	}

	//type := "e2-custom-8-16384"
	image := fmt.Sprintf("projects/ubuntu-os-cloud/global/images/%s", args.Image)
	startup := "chmod a+rwt /tmp"
	inst = &compute.Instance{
		Name:        args.Name,
		Description: fmt.Sprintf("%s-%s", args.Name, args.Image),
		MachineType: fmt.Sprintf("projects/%s/zones/%s/machineTypes/%s",
			args.Project, args.Region, args.Type),
		Zone: fmt.Sprintf("projects/%s/zones/%s", args.Project, args.Region),
		Tags: &compute.Tags{
			Items: []string{"vegorov"},
		},
		Disks: []*compute.AttachedDisk{
			{
				AutoDelete: true,
				Boot:       true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskType:    "projects/npa-development/zones/us-west1-b/diskTypes/pd-ssd",
					SourceImage: image,
					DiskSizeGb:  200,
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
				Subnetwork: "projects/npa-development/regions/us-west1/subnetworks/vpc-npa-priv-vpn-subnet",
			},
			{
				Subnetwork: "projects/ns-npe-shared-vpc/regions/us-west1/subnetworks/shared-vpc-ns-usw1",
				//Subnetwork: "projects/npa-development/regions/us-west1/subnetworks/default"
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				&compute.MetadataItems{
					Key:   "user-data",
					Value: &user_data,
				},
				&compute.MetadataItems{
					Key:   "startup-script",
					Value: &startup,
				},
			},
		},
	}

	log.Printf("crating instance %s", args.Name)
	op, err := service.Instances.Insert(args.Project, args.Region, inst).Do()
	wait(args, op, err)

	inst, err = service.Instances.Get(args.Project, args.Region, args.Name).Do()

	log.Printf("instance ip %s - %s",
		inst.NetworkInterfaces[0].NetworkIP,
		inst.NetworkInterfaces[0].AccessConfigs[0].NatIP, inst)
	updateDNS(args, inst.NetworkInterfaces[0].AccessConfigs[0].NatIP)
}

func main() {
	args := Args{}

	util.GetFlags(&args, "gcp")

	if args.Verbose {
		util.Dump("args", args)
	}

	scopes := strings.Join([]string{
		//compute.DevstorageFullControlScope,
		compute.CloudPlatformScope,
		dns.CloudPlatformScope,
	}, " ")

	config := &oauth2.Config{
		ClientID:     args.ClientID,
		ClientSecret: args.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{scopes},
	}

	ctx := context.Background()
	c := newOAuthClient(ctx, config)
	args.client = c
	create(c, &args)
}

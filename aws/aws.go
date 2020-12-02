package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"gotools/util"
	"log"
	"os"
	"os/user"
	"path"
	"reflect"
	"strings"
	"time"
	"unsafe"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/route53"
)

type Args struct {
	Region    string `json:"region"`
	Image     string `json:"image"`
	Type      string `json:"type"`
	Key       string `json:"key"`
	Name      string `json:"name"`
	Owner     string `json:"owner"`
	User      string `json:"user"`
	Subnet    string `json:"subnet"`
	Nic       string `json:"nic"`
	Group     string `json:"group"`
	Zone      string `json:"zone"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Token     string `json:"token"`
	Verbose   bool   `json:"verbose"`
}

func load(a *Args) {
	user, err := user.Current()
	if err != nil {
		return
	}
	a.User = user.Username
	a.Owner = user.Name
	fn := path.Join(user.HomeDir, ".aws.json")

	if f, err := os.Open(fn); err == nil {
		defer f.Close()
		p := json.NewDecoder(f)
		p.Decode(a)
	}

}

func parse(a *Args) {
	v := reflect.ValueOf(a).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Name
		tag := f.Tag.Get("json")
		if tag != "" {
			name = strings.Split(tag, ",")[0]
		}
		vf := v.Field(i)
		p := (*string)(unsafe.Pointer(vf.UnsafeAddr()))
		flag.StringVar(p, name, *p, "")

	}
	flag.Parse()
}

func update_dns(sess *session.Session, args *Args, ip string) {
	if args.Zone == "" || ip == "" {
		return
	}

	dns := route53.New(sess)

	zones, err := dns.ListHostedZonesByName(
		&route53.ListHostedZonesByNameInput{
			DNSName: &args.Zone,
		})
	if err != nil {
		log.Printf("Error %s", err)
		return
	}

	id := ""
	for _, z := range zones.HostedZones {
		if *z.Name == args.Zone {
			id = *z.Id
		}
	}

	if args.Type == "-" || args.Type == "none" {
		_, err = dns.ChangeResourceRecordSets(
			&route53.ChangeResourceRecordSetsInput{
				ChangeBatch: &route53.ChangeBatch{
					Changes: []*route53.Change{
						{
							Action: aws.String("DELETE"),
							ResourceRecordSet: &route53.ResourceRecordSet{
								Name: aws.String(args.Name + "." + args.Zone),
								Type: aws.String("A"),
								TTL:  aws.Int64(30),
								ResourceRecords: []*route53.ResourceRecord{
									{
										Value: aws.String(ip),
									},
								},
							},
						},
					},
				},
				HostedZoneId: aws.String(id),
			})

		if err != nil {
			log.Printf("Error %s", err)
			return
		}
	}

	name := args.Name + "." + strings.TrimRight(args.Zone, ".")
	log.Printf("Updating record %s : %s", name, ip)

	_, err = dns.ChangeResourceRecordSets(
		&route53.ChangeResourceRecordSetsInput{
			ChangeBatch: &route53.ChangeBatch{
				Changes: []*route53.Change{
					{
						Action: aws.String("UPSERT"),
						ResourceRecordSet: &route53.ResourceRecordSet{
							Name: aws.String(args.Name + "." + args.Zone),
							Type: aws.String("A"),
							TTL:  aws.Int64(30),
							ResourceRecords: []*route53.ResourceRecord{
								{
									Value: aws.String(ip),
								},
							},
						},
					},
				},
			},
			HostedZoneId: aws.String(id),
		})

	if err != nil {
		log.Printf("Error %s", err)
		return
	}
}

func cleanup(args Args, svc *ec2.EC2) (ip string) {

	filter := []*ec2.Filter{
		&ec2.Filter{
			Name: aws.String("tag:Name"),
			Values: []*string{
				&args.Name,
			},
		},
	}

	desc, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: filter,
	})

	if err != nil {
		log.Printf("Error enumerating %s", err)
		return
	}

	if len(desc.Reservations) > 0 {
		ids := []*string{}
		nids := []*string{}

		for _, r := range desc.Reservations {
			for _, i := range r.Instances {
				if i.PublicIpAddress != nil {
					ip = *i.PublicIpAddress
				}
				log.Printf("Cleaning up %s: %s", *i.InstanceId, ip)
				ids = append(ids, i.InstanceId)
				for _, n := range i.NetworkInterfaces {
					nids = append(nids, n.NetworkInterfaceId)
				}
			}
		}
		_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: ids,
		})
		if err != nil {
			log.Printf("Error deleting %s", err)
			return
		}
		err = svc.WaitUntilInstanceTerminated(
			&ec2.DescribeInstancesInput{
				InstanceIds: ids,
			})
		for _, n := range nids {
			_, err = svc.DeleteNetworkInterface(&ec2.DeleteNetworkInterfaceInput{
				NetworkInterfaceId: n,
			})
			log.Printf("Cleaning up %s", *n)
		}
	}

	intfs, err := svc.DescribeNetworkInterfaces(
		&ec2.DescribeNetworkInterfacesInput{
			Filters: filter,
		})

	for _, n := range intfs.NetworkInterfaces {
		_, err = svc.DeleteNetworkInterface(&ec2.DeleteNetworkInterfaceInput{
			NetworkInterfaceId: n.NetworkInterfaceId,
		})
		log.Printf("Cleaning up %s => %s", *n.NetworkInterfaceId, err)
	}
	return
}

func add_nic(args Args, svc *ec2.EC2, inst *string) (nic *string) {
	if args.Nic == "" {
		return
	}

	ni, err := svc.CreateNetworkInterface(&ec2.CreateNetworkInterfaceInput{
		Groups:   []*string{aws.String(args.Group)},
		SubnetId: aws.String(args.Nic),
	})
	if err != nil {
		log.Printf("Could not create network interface %s", err)
		return
	}

	_, err = svc.AttachNetworkInterface(&ec2.AttachNetworkInterfaceInput{
		NetworkInterfaceId: ni.NetworkInterface.NetworkInterfaceId,
		InstanceId:         inst,
		DeviceIndex:        aws.Int64(1),
	})
	if err != nil {
		log.Printf("Could not attach interface %s", err)
		return
	}
	nic = ni.NetworkInterface.NetworkInterfaceId
	return
}

func main() {

	args := Args{}

	util.GetFlags(&args, "aws")

	if args.Verbose {
		util.Dump("args", args)
	}

	cred := credentials.NewStaticCredentials(
		args.AccessKey, args.SecretKey, args.Token)
	cfg := &aws.Config{
		Region:      aws.String(args.Region),
		Credentials: cred,
	}
	sess := session.Must(session.NewSession(cfg))
	svc := ec2.New(sess)

	oldip := cleanup(args, svc)

	if args.Type == "-" || args.Type == "none" {
		update_dns(sess, &args, oldip)
		return
	}

	if !strings.HasPrefix(args.Image, "ami") {
		di := ec2.DescribeImagesInput{
			Filters: []*ec2.Filter{
				&ec2.Filter{
					Name: aws.String("tag:Name"),
					Values: []*string{
						&args.Name,
					},
				},
			},
		}
		res, err := svc.DescribeImages(&di)
		if err != nil {
			log.Printf("Could not find image %s", err)
			return
		}
		args.Image = *res.Images[0].ImageId
	}

	tags := []*ec2.Tag{
		&ec2.Tag{
			Key:   aws.String("Name"),
			Value: aws.String(args.Name),
		},
		&ec2.Tag{
			Key:   aws.String("Owner"),
			Value: aws.String(args.Owner),
		},
		&ec2.Tag{
			Key:   aws.String("auto:start"),
			Value: aws.String("* * * * *"),
		},
		&ec2.Tag{
			Key:   aws.String("Active"),
			Value: aws.String("True"),
		},
		&ec2.Tag{
			Key:   aws.String("Expire"),
			Value: aws.String("2029-09-23"),
		},
	}

	params := &ec2.RunInstancesInput{
		ImageId:      aws.String(args.Image),
		InstanceType: aws.String(args.Type),
		MaxCount:     aws.Int64(1),
		MinCount:     aws.Int64(1),
		KeyName:      aws.String(args.Key),
		NetworkInterfaces: []*ec2.InstanceNetworkInterfaceSpecification{
			&ec2.InstanceNetworkInterfaceSpecification{
				//AssociatePublicIpAddress: aws.Bool(true),
				SubnetId:    aws.String(args.Subnet),
				DeviceIndex: aws.Int64(0),
				Groups:      []*string{aws.String(args.Group)},
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			&ec2.TagSpecification{
				ResourceType: aws.String("instance"),
				Tags:         tags,
			},
		},
		BlockDeviceMappings: []*ec2.BlockDeviceMapping{
			&ec2.BlockDeviceMapping{
				DeviceName: aws.String("/dev/sda1"),
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize: aws.Int64(500),
				},
			},
		},
	}

	runResult, err := svc.RunInstances(params)
	if err != nil {
		log.Printf("Could not create instance %s", err)
		return
	}

	inst := runResult.Instances[0]
	instanceID := *inst.InstanceId

	dii := ec2.DescribeInstancesInput{
		InstanceIds: []*string{&instanceID},
	}
	err = svc.WaitUntilInstanceExists(&dii)
	if err != nil {
		log.Printf("Error waiting %s : %s", instanceID, err)
	}

	ip := ""
	for {
		desc, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
			InstanceIds: []*string{&instanceID},
		})
		if err != nil {
			log.Printf("Error describing %s : %s", instanceID, err)
		}
		if *desc.Reservations[0].Instances[0].State.Code > 0 {
			fmt.Printf("%s\n", desc)
			if desc.Reservations[0].Instances[0].PublicIpAddress != nil {
				ip = *desc.Reservations[0].Instances[0].PublicIpAddress
			} else {
				ip = *desc.Reservations[0].Instances[0].PrivateIpAddress
			}
			log.Printf("Created instance %s: %s", instanceID, ip)
			break
		}
		time.Sleep(time.Second)
	}

	nic := add_nic(args, svc, inst.InstanceId)

	if nic != nil {
		tags := &ec2.CreateTagsInput{
			Resources: []*string{
				nic,
			},
			Tags: tags,
		}
		_, err = svc.CreateTags(tags)
		if err != nil {
			log.Printf("Could not create tags for instance %s : %s", instanceID, err)
		}
	}

	update_dns(sess, &args, ip)

}

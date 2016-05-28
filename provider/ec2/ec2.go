package ec2

import "github.com/LunaNode/cloug/provider/common"
import "github.com/LunaNode/cloug/service/compute"
import "github.com/LunaNode/cloug/utils"

import "github.com/aws/aws-sdk-go/aws"
import "github.com/aws/aws-sdk-go/aws/credentials"
import "github.com/aws/aws-sdk-go/aws/session"
import "github.com/aws/aws-sdk-go/service/ec2"

import "encoding/base64"
import "errors"
import "fmt"
import "strings"

const DEFAULT_NAME = "cloug"
const DEFAULT_REGION = "us-west-2"

func encodeID(id string, region string) string {
	return region + ":" + id
}

func decodeID(id string) (string, string) {
	parts := strings.Split(id, ":")
	if len(parts) < 2 {
		return parts[0], DEFAULT_REGION
	} else {
		return parts[1], parts[0]
	}
}

func String(str *string) string {
	if str == nil {
		return ""
	} else {
		return *str
	}
}

func Bool(b *bool) bool {
	return b != nil && *b
}

func Int64(x *int64) int64 {
	if x == nil {
		return 0
	} else {
		return *x
	}
}

type EC2 struct {
	Session *session.Session
}

func MakeEC2(keyID string, secretKey string, apiToken string) (*EC2, error) {
	creds := credentials.NewStaticCredentials(keyID, secretKey, apiToken)
	config := &aws.Config{Credentials: creds}
	e := &EC2{Session: session.New(config)}
	return e, nil
}

func (e *EC2) ComputeService() compute.Service {
	return e
}

func (e *EC2) getService(region string) *ec2.EC2 {
	return ec2.New(e.Session, &aws.Config{Region: aws.String(region)})
}

func (e *EC2) mapInstanceStatus(state string) compute.InstanceStatus {
	if state == "running" {
		return compute.StatusOnline
	} else if state == "stopped" {
		return compute.StatusOffline
	} else {
		return compute.InstanceStatus(strings.ToLower(state))
	}
}

func (e *EC2) mapInstance(instance *ec2.Instance, region string) *compute.Instance {
	return &compute.Instance{
		ID:     encodeID(String(instance.InstanceId), region),
		Status: e.mapInstanceStatus(String(instance.State.Name)),
		IP:     String(instance.PublicIpAddress),
	}
}

func (e *EC2) CreateInstance(instance *compute.Instance) (*compute.Instance, error) {
	imageID, err := common.GetMatchingImageID(e, &instance.Image)
	if err != nil {
		return nil, err
	}
	flavorID, err := common.GetMatchingFlavorID(e, &instance.Flavor)
	if err != nil {
		return nil, err
	}

	region := DEFAULT_REGION
	if instance.Region != "" {
		region = instance.Region
	}
	svc := e.getService(region)

	opts := ec2.RunInstancesInput{
		ImageId:      aws.String(imageID),
		InstanceType: aws.String(flavorID),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
	}

	password := instance.Password
	if password == "" {
		password = utils.Uid(16)
	}
	userData := "#cloud-config\npassword: " + password + "\nchpasswd: { expire: False }\nssh_pwauth: True\n"
	opts.UserData = aws.String(base64.StdEncoding.EncodeToString([]byte(userData)))

	if instance.Flavor.DiskGB > 0 {
		opts.BlockDeviceMappings = []*ec2.BlockDeviceMapping{
			{
				Ebs: &ec2.EbsBlockDevice{
					VolumeSize:          aws.Int64(int64(instance.Flavor.DiskGB)),
					DeleteOnTermination: aws.Bool(true),
					VolumeType:          aws.String("gp2"),
				},
			},
		}
	}

	if len(instance.PublicKey.Key) > 0 {
		keyName := utils.Uid(8)
		_, err := svc.ImportKeyPair(&ec2.ImportKeyPairInput{
			KeyName:           aws.String(keyName),
			PublicKeyMaterial: instance.PublicKey.Key,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to import public key: %v", err)
		}
		defer svc.DeleteKeyPair(&ec2.DeleteKeyPairInput{
			KeyName: aws.String(keyName),
		})
		opts.KeyName = aws.String(keyName)
	}

	res, err := svc.RunInstances(&opts)
	if err != nil {
		return nil, err
	} else if len(res.Instances) != 1 {
		return nil, fmt.Errorf("attempted to provision a single instance, but reservation contains %d instances", len(res.Instances))
	}
	resInstance := res.Instances[0]

	return &compute.Instance{
		ID:     encodeID(String(resInstance.InstanceId), region),
		Status: e.mapInstanceStatus(String(resInstance.State.Name)),
	}, nil
}

func (e *EC2) regionAction(encodedID string, f func(id string, svc *ec2.EC2) error) error {
	id, region := decodeID(encodedID)
	svc := e.getService(region)
	return f(id, svc)
}

func (e *EC2) DeleteInstance(instanceID string) error {
	return e.regionAction(instanceID, func(id string, svc *ec2.EC2) error {
		_, err := svc.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{aws.String(id)},
		})
		return err
	})
}

func (e *EC2) ListInstances() ([]*compute.Instance, error) {
	return nil, fmt.Errorf("operation not supported")
}

func (e *EC2) getInstance(id string, svc *ec2.EC2) (*ec2.Instance, error) {
	resp, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(id)},
	})
	if err != nil {
		return nil, err
	} else if len(resp.Reservations) != 1 || len(resp.Reservations[0].Instances) != 1 {
		return nil, fmt.Errorf("DescribeInstances did not return one reservation with one instance")
	} else {
		return resp.Reservations[0].Instances[0], nil
	}
}

func (e *EC2) GetInstance(instanceID string) (*compute.Instance, error) {
	id, region := decodeID(instanceID)
	svc := e.getService(region)
	instance, err := e.getInstance(id, svc)
	if err != nil {
		return nil, err
	} else {
		return e.mapInstance(instance, region), nil
	}
}

func (e *EC2) StartInstance(instanceID string) error {
	return e.regionAction(instanceID, func(id string, svc *ec2.EC2) error {
		_, err := svc.StartInstances(&ec2.StartInstancesInput{
			InstanceIds: []*string{aws.String(id)},
		})
		return err
	})
}

func (e *EC2) StopInstance(instanceID string) error {
	return e.regionAction(instanceID, func(id string, svc *ec2.EC2) error {
		_, err := svc.StopInstances(&ec2.StopInstancesInput{
			InstanceIds: []*string{aws.String(id)},
		})
		return err
	})
}

func (e *EC2) RebootInstance(instanceID string) error {
	return e.regionAction(instanceID, func(id string, svc *ec2.EC2) error {
		_, err := svc.RebootInstances(&ec2.RebootInstancesInput{
			InstanceIds: []*string{aws.String(id)},
		})
		return err
	})
}

func (e *EC2) CreateImage(imageTemplate *compute.Image) (*compute.Image, error) {
	if imageTemplate.SourceInstance != "" {
		instanceID, region := decodeID(imageTemplate.SourceInstance)
		svc := e.getService(region)

		name := DEFAULT_NAME
		if imageTemplate.Name != "" {
			name = imageTemplate.Name
		}

		resp, err := svc.CreateImage(&ec2.CreateImageInput{
			InstanceId: aws.String(instanceID),
			Name:       aws.String(name),
		})
		if err != nil {
			return nil, err
		} else {
			return &compute.Image{
				ID:             encodeID(String(resp.ImageId), region),
				Name:           imageTemplate.Name,
				SourceInstance: imageTemplate.SourceInstance,
			}, nil
		}
	} else if imageTemplate.SourceURL != "" {
		return nil, errors.New("CreateImage from URL is not implemented")
	} else {
		return nil, errors.New("neither source instance nor source URL is set")
	}
}

func (e *EC2) mapImage(apiImage *ec2.Image, region string) *compute.Image {
	image := &compute.Image{
		ID:     encodeID(String(apiImage.ImageId), region),
		Name:   String(apiImage.Name),
		Public: Bool(apiImage.Public),
	}

	if len(apiImage.BlockDeviceMappings) > 0 {
		image.Size = Int64(apiImage.BlockDeviceMappings[0].Ebs.VolumeSize)
	}

	if String(apiImage.State) == "available" {
		image.Status = compute.ImageAvailable
	} else if String(apiImage.State) == "error" || String(apiImage.State) == "failed" || String(apiImage.State) == "invalid" {
		image.Status = "error"
	} else {
		image.Status = compute.ImagePending
	}

	return image
}

func (e *EC2) FindImage(image *compute.Image) (string, error) {
	return "", errors.New("operation not supported")
}

func (e *EC2) ListImages() ([]*compute.Image, error) {
	return nil, errors.New("operation not supported")
}

func (e *EC2) GetImage(imageID string) (*compute.Image, error) {
	id, region := decodeID(imageID)
	svc := e.getService(region)
	resp, err := svc.DescribeImages(&ec2.DescribeImagesInput{
		ImageIds: []*string{aws.String(id)},
	})
	if err != nil {
		return nil, err
	} else if len(resp.Images) != 1 {
		return nil, fmt.Errorf("DescribeImages returned %d images, but expected a single image", len(resp.Images))
	} else {
		return e.mapImage(resp.Images[0], region), nil
	}
}

func (e *EC2) DeleteImage(imageID string) error {
	return fmt.Errorf("operation not supported")
}

func (e *EC2) ListFlavors() ([]*compute.Flavor, error) {
	return []*compute.Flavor{
		{
			ID:       "t2.nano",
			Name:     "t2.nano",
			MemoryMB: 512,
			NumCores: 1,
		},
		{
			ID:       "t2.micro",
			Name:     "t2.micro",
			MemoryMB: 1024,
			NumCores: 1,
		},
		{
			ID:       "t2.small",
			Name:     "t2.small",
			MemoryMB: 2048,
			NumCores: 1,
		},
		{
			ID:       "t2.medium",
			Name:     "t2.medium",
			MemoryMB: 4096,
			NumCores: 2,
		},
		{
			ID:       "t2.large",
			Name:     "t2.large",
			MemoryMB: 8192,
			NumCores: 2,
		},
		{
			ID:       "m4.large",
			Name:     "m4.large",
			MemoryMB: 8192,
			NumCores: 2,
		},
		{
			ID:       "m4.xlarge",
			Name:     "m4.xlarge",
			MemoryMB: 16384,
			NumCores: 4,
		},
		{
			ID:       "m4.2xlarge",
			Name:     "m4.2xlarge",
			MemoryMB: 32768,
			NumCores: 6,
		},
		{
			ID:       "m4.4xlarge",
			Name:     "m4.4xlarge",
			MemoryMB: 65536,
			NumCores: 16,
		},
		{
			ID:       "m4.10xlarge",
			Name:     "m4.10xlarge",
			MemoryMB: 163840,
			NumCores: 20,
		},
		{
			ID:       "m3.medium",
			Name:     "m3.medium",
			MemoryMB: 3840,
			NumCores: 1,
		},
		{
			ID:       "m3.large",
			Name:     "m3.large",
			MemoryMB: 7680,
			NumCores: 2,
		},
		{
			ID:       "m3.xlarge",
			Name:     "m3.xlarge",
			MemoryMB: 15360,
			NumCores: 4,
		},
		{
			ID:       "m3.2xlarge",
			Name:     "m3.2xlarge",
			MemoryMB: 30720,
			NumCores: 8,
		},
	}, nil
}

func (e *EC2) FindFlavor(flavor *compute.Flavor) (string, error) {
	flavors, err := e.ListFlavors()
	if err != nil {
		return "", fmt.Errorf("error listing flavors: %v", err)
	}
	return common.MatchFlavor(flavor, flavors), nil
}

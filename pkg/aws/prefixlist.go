package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	// prefixListMaxEntries is the maximum number of entries a created prefix list can have.
	// This number counts against a resources quota (regardless of how many actual entries exist)
	// when it is referenced. We're setting the max here to 45 for now so we stay below the
	// default "Routes per route table" quota of 50.
	prefixListMaxEntries = 45
	prefixListNameFilter = "prefix-list-name"
)

type PrefixLists struct {
	client *ec2.EC2
}

func (t *PrefixLists) Apply(ctx context.Context, name string, tags map[string]string) (string, error) {
	prefixLists, err := t.get(ctx, name)
	if err != nil {
		return "", err
	}

	if len(prefixLists) == 1 {
		return *prefixLists[0].PrefixListArn, nil
	}

	if len(prefixLists) > 1 {
		return "", fmt.Errorf(
			"found unexpected number: %d of prefix lists for cluster %s",
			len(prefixLists),
			name,
		)
	}

	return t.create(ctx, name, tags)
}

func (t *PrefixLists) Delete(ctx context.Context, name string) error {
	prefixLists, err := t.get(ctx, name)
	if err != nil {
		return err
	}

	if len(prefixLists) == 0 {
		return nil
	}

	if len(prefixLists) > 1 {
		return fmt.Errorf(
			"found unexpected number: %d of prefix lists for cluster %s",
			len(prefixLists),
			name,
		)
	}

	id := prefixLists[0].PrefixListId
	_, err = t.client.DeleteManagedPrefixList(&ec2.DeleteManagedPrefixListInput{
		PrefixListId: id,
	})

	return err
}

func (t *PrefixLists) get(ctx context.Context, name string) ([]*ec2.ManagedPrefixList, error) {
	input := &ec2.DescribeManagedPrefixListsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String(prefixListNameFilter),
				Values: aws.StringSlice([]string{GetPrefixListName(name)}),
			},
		},
	}

	out, err := t.client.DescribeManagedPrefixListsWithContext(ctx, input)
	if err != nil {
		return nil, err
	}

	return out.PrefixLists, nil
}

func (t *PrefixLists) create(ctx context.Context, name string, tags map[string]string) (string, error) {
	input := &ec2.CreateManagedPrefixListInput{
		AddressFamily:  awssdk.String("IPv4"),
		MaxEntries:     awssdk.Int64(prefixListMaxEntries),
		PrefixListName: awssdk.String(GetPrefixListName(name)),
		TagSpecifications: []*ec2.TagSpecification{
			{
				ResourceType: aws.String(ec2.ResourceTypePrefixList),
				Tags:         getEc2Tags(tags),
			},
		},
	}
	out, err := t.client.CreateManagedPrefixListWithContext(ctx, input)
	if err != nil {
		return "", err
	}

	return *out.PrefixList.PrefixListArn, nil
}

func GetPrefixListName(name string) string {
	return fmt.Sprintf("%s-tgw-prefixlist", name)
}

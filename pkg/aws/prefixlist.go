package aws

import (
	"context"
	"fmt"
	"net"

	"github.com/aws/aws-sdk-go/aws"
	awssdk "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pkg/errors"

	"github.com/aws-resolver-rules-operator/pkg/resolver"
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

func (p *PrefixLists) ApplyEntry(ctx context.Context, entry resolver.PrefixListEntry) error {
	prefixListID, err := GetARNResourceID(entry.PrefixListARN)
	if err != nil {
		return errors.WithStack(err)
	}

	err = validateCIDR(entry.CIDR)
	if err != nil {
		return errors.WithStack(err)
	}

	exists, err := p.entryExists(ctx, prefixListID, entry)
	if err != nil {
		return errors.WithStack(err)
	}

	if exists {
		return nil
	}

	return p.createEntry(ctx, prefixListID, entry)
}

func (p *PrefixLists) DeleteEntry(ctx context.Context, entry resolver.PrefixListEntry) error {
	prefixListID, err := GetARNResourceID(entry.PrefixListARN)
	if err != nil {
		return errors.WithStack(err)
	}

	err = validateCIDR(entry.CIDR)
	if err != nil {
		return errors.WithStack(err)
	}

	exists, err := p.entryExists(ctx, prefixListID, entry)
	if err != nil {
		return errors.WithStack(err)
	}

	if !exists {
		return nil
	}

	return p.deleteEntry(ctx, prefixListID, entry)
}

func (t *PrefixLists) Delete(ctx context.Context, name string) error {
	prefixLists, err := t.get(ctx, name)
	if err != nil {
		return errors.WithStack(err)
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

	return errors.WithStack(err)
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

func (p *PrefixLists) createEntry(ctx context.Context, prefixListID string, entry resolver.PrefixListEntry) error {
	_, err := p.client.ModifyManagedPrefixListWithContext(ctx, &ec2.ModifyManagedPrefixListInput{
		PrefixListId: awssdk.String(prefixListID),
		AddEntries: []*ec2.AddPrefixListEntry{
			{
				Cidr:        awssdk.String(entry.CIDR),
				Description: awssdk.String(entry.Description),
			},
		},
	})

	return errors.WithStack(err)
}

func (p *PrefixLists) entryExists(ctx context.Context, prefixListID string, entry resolver.PrefixListEntry) (bool, error) {
	out, err := p.client.GetManagedPrefixListEntriesWithContext(ctx, &ec2.GetManagedPrefixListEntriesInput{
		PrefixListId: awssdk.String(prefixListID),
		MaxResults:   awssdk.Int64(100),
	})
	if err != nil {
		return false, err
	}

	for _, e := range out.Entries {
		if *e.Cidr == entry.CIDR {
			if *e.Description != entry.Description {
				return false, errors.New("found conflicting prefix list entry")
			}

			return true, nil
		}
	}

	return false, nil
}

func (p *PrefixLists) deleteEntry(ctx context.Context, prefixListID string, entry resolver.PrefixListEntry) error {
	_, err := p.client.ModifyManagedPrefixListWithContext(ctx, &ec2.ModifyManagedPrefixListInput{
		PrefixListId: awssdk.String(prefixListID),
		RemoveEntries: []*ec2.RemovePrefixListEntry{
			{
				Cidr: awssdk.String(entry.CIDR),
			},
		},
	})

	return errors.WithStack(err)
}

func GetPrefixListName(name string) string {
	return fmt.Sprintf("%s-tgw-prefixlist", name)
}

func validateCIDR(cidr string) error {
	_, _, err := net.ParseCIDR(cidr)
	return errors.WithStack(err)
}

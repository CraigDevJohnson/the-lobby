package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Dynamo keeps Members and Sign-in sessions in one DynamoDB table, told apart
// by a prefix on the key. Sessions carry expires_at, which the table's TTL
// setting uses to remove them; Members never expire.
type Dynamo struct {
	client *dynamodb.Client
	table  string
}

func NewDynamo(client *dynamodb.Client, table string) *Dynamo {
	return &Dynamo{client: client, table: table}
}

type memberItem struct {
	ID      string   `dynamodbav:"id"`
	Kind    string   `dynamodbav:"kind"`
	Email   string   `dynamodbav:"email"`
	Tools   []string `dynamodbav:"tools"`
	AddedAt string   `dynamodbav:"added_at"`
}

type sessionItem struct {
	ID        string `dynamodbav:"id"`
	Kind      string `dynamodbav:"kind"`
	Email     string `dynamodbav:"email"`
	CreatedAt string `dynamodbav:"created_at"`
	ExpiresAt int64  `dynamodbav:"expires_at"`
}

func memberKey(email string) string { return "member#" + NormalizeEmail(email) }
func sessionKey(id string) string   { return "session#" + id }

func (d *Dynamo) get(ctx context.Context, key string, out any) error {
	res, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(d.table),
		Key:            map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: key}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return fmt.Errorf("get %s: %w", key, err)
	}
	if res.Item == nil {
		return ErrNotFound
	}
	return attributevalue.UnmarshalMap(res.Item, out)
}

func (d *Dynamo) put(ctx context.Context, item any) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{TableName: aws.String(d.table), Item: av})
	return err
}

func (d *Dynamo) del(ctx context.Context, key string) error {
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(d.table),
		Key:       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: key}},
	})
	return err
}

func (d *Dynamo) GetMember(ctx context.Context, email string) (Member, error) {
	var it memberItem
	if err := d.get(ctx, memberKey(email), &it); err != nil {
		return Member{}, err
	}
	added, _ := time.Parse(time.RFC3339, it.AddedAt)
	return Member{Email: it.Email, Tools: it.Tools, AddedAt: added}, nil
}

func (d *Dynamo) PutMember(ctx context.Context, m Member) error {
	m.Email = NormalizeEmail(m.Email)
	if m.AddedAt.IsZero() {
		m.AddedAt = time.Now().UTC()
	}
	return d.put(ctx, memberItem{ID: memberKey(m.Email), Kind: "member", Email: m.Email, Tools: m.Tools, AddedAt: m.AddedAt.Format(time.RFC3339)})
}

func (d *Dynamo) DeleteMember(ctx context.Context, email string) error {
	return d.del(ctx, memberKey(email))
}

func (d *Dynamo) ListMembers(ctx context.Context) ([]Member, error) {
	var out []Member
	p := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		FilterExpression:          aws.String("kind = :k"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":k": &types.AttributeValueMemberS{Value: "member"}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		var items []memberItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, err
		}
		for _, it := range items {
			added, _ := time.Parse(time.RFC3339, it.AddedAt)
			out = append(out, Member{Email: it.Email, Tools: it.Tools, AddedAt: added})
		}
	}
	return out, nil
}

func (d *Dynamo) GetSession(ctx context.Context, id string) (Session, error) {
	var it sessionItem
	if err := d.get(ctx, sessionKey(id), &it); err != nil {
		return Session{}, err
	}
	exp := time.Unix(it.ExpiresAt, 0)
	if !exp.After(time.Now()) {
		// TTL deletion lags; treat an expired item as gone.
		return Session{}, ErrNotFound
	}
	created, _ := time.Parse(time.RFC3339, it.CreatedAt)
	return Session{ID: id, Email: it.Email, CreatedAt: created, ExpiresAt: exp}, nil
}

func (d *Dynamo) PutSession(ctx context.Context, s Session) error {
	return d.put(ctx, sessionItem{ID: sessionKey(s.ID), Kind: "session", Email: s.Email, CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339), ExpiresAt: s.ExpiresAt.Unix()})
}

func (d *Dynamo) DeleteSession(ctx context.Context, id string) error {
	return d.del(ctx, sessionKey(id))
}

// IsNotFound reports whether err means the item does not exist.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

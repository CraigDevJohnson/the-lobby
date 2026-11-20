package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Dynamo keeps Members, Sign-in sessions and Members' Let's Plan Soccer
// calendars in one DynamoDB table, told apart by a prefix on the key.
// Sessions carry expires_at, which the table's TTL setting uses to remove
// them; nothing else expires. A Member's calendar is items of its own,
// soccer#<email>, soccerlink#<secret>, google#<email> and icloud#<email>,
// never part of
// member#<email>, which `site member add` writes whole.
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

type soccerItem struct {
	ID     string           `dynamodbav:"id"`
	Kind   string           `dynamodbav:"kind"`
	Email  string           `dynamodbav:"email"`
	Teams  []RememberedTeam `dynamodbav:"teams"`
	Secret string           `dynamodbav:"secret"`
}

type soccerLinkItem struct {
	ID    string `dynamodbav:"id"`
	Kind  string `dynamodbav:"kind"`
	Email string `dynamodbav:"email"`
}

type googleItem struct {
	ID         string `dynamodbav:"id"`
	Kind       string `dynamodbav:"kind"`
	Email      string `dynamodbav:"email"`
	Account    string `dynamodbav:"account"`
	Token      string `dynamodbav:"token"`
	CalendarID string `dynamodbav:"calendar_id"`
}

func googleKey(email string) string { return "google#" + NormalizeEmail(email) }

type icloudItem struct {
	ID         string `dynamodbav:"id"`
	Kind       string `dynamodbav:"kind"`
	Email      string `dynamodbav:"email"`
	Account    string `dynamodbav:"account"`
	Password   string `dynamodbav:"password"`
	Home       string `dynamodbav:"home"`
	CalendarID string `dynamodbav:"calendar_id"`
	// The daily update's switch and the UIDs it has written (ADR 0015).
	// Connections made before it have neither, which reads as off and none.
	Daily   bool     `dynamodbav:"daily"`
	Written []string `dynamodbav:"written"`
	// Each written game's start and field, by UID (ADR 0017). An item
	// written before these were kept has no times, which reads as a nil map
	// beside its list.
	Times map[string]GameTime `dynamodbav:"times"`
}

func (it icloudItem) connection() ICloud {
	return ICloud{Email: it.Email, Account: it.Account, Password: it.Password, Home: it.Home, CalendarID: it.CalendarID, Daily: it.Daily, Written: it.Written, Times: it.Times}
}

func icloudKey(email string) string { return "icloud#" + NormalizeEmail(email) }

func memberKey(email string) string      { return "member#" + NormalizeEmail(email) }
func sessionKey(id string) string        { return "session#" + id }
func soccerKey(email string) string      { return "soccer#" + NormalizeEmail(email) }
func soccerLinkKey(secret string) string { return "soccerlink#" + secret }

func (d *Dynamo) get(ctx context.Context, key string, out any) error {
	res, err := d.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(d.table),
		Key:            map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: key}},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		// The error is logged, so it names only the kind of record: a
		// session's key is the sign-in cookie's value and a Member's is their
		// address.
		kind, _, _ := strings.Cut(key, "#")
		return fmt.Errorf("get %s: %w", kind, err)
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
	if err := d.del(ctx, memberKey(email)); err != nil {
		return err
	}
	if err := d.del(ctx, googleKey(email)); err != nil {
		return err
	}
	if err := d.del(ctx, icloudKey(email)); err != nil {
		return err
	}
	s, err := d.GetSoccer(ctx, email)
	switch {
	case errors.Is(err, ErrNotFound):
		return nil
	case err != nil:
		return err
	}
	// The link first, so it stops even if the rest fails.
	if s.Secret != "" {
		if err := d.del(ctx, soccerLinkKey(s.Secret)); err != nil {
			return err
		}
	}
	return d.del(ctx, soccerKey(email))
}

func (d *Dynamo) ListMembers(ctx context.Context) ([]Member, error) {
	var out []Member
	// Strongly consistent, like get, so Lobby Admin's answer to a change
	// shows that change.
	p := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		ConsistentRead:            aws.Bool(true),
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

// ListSessions reads the whole table, as ListMembers does, keeping the
// sessions that haven't expired: TTL deletion lags, as GetSession allows.
func (d *Dynamo) ListSessions(ctx context.Context) ([]Session, error) {
	var out []Session
	now := time.Now()
	// Strongly consistent, so Sign out everywhere also finds a session started
	// a moment before.
	p := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		ConsistentRead:            aws.Bool(true),
		FilterExpression:          aws.String("kind = :k"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":k": &types.AttributeValueMemberS{Value: "session"}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			// Like get, the error names only the kind of record.
			return nil, fmt.Errorf("list sessions: %w", err)
		}
		var items []sessionItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, fmt.Errorf("list sessions: %w", err)
		}
		for _, it := range items {
			exp := time.Unix(it.ExpiresAt, 0)
			if !exp.After(now) {
				continue
			}
			created, _ := time.Parse(time.RFC3339, it.CreatedAt)
			out = append(out, Session{ID: strings.TrimPrefix(it.ID, "session#"), Email: it.Email, CreatedAt: created, ExpiresAt: exp})
		}
	}
	return out, nil
}

func (d *Dynamo) GetSoccer(ctx context.Context, email string) (Soccer, error) {
	var it soccerItem
	if err := d.get(ctx, soccerKey(email), &it); err != nil {
		return Soccer{}, err
	}
	return Soccer{Email: it.Email, Teams: it.Teams, Secret: it.Secret}, nil
}

func (d *Dynamo) PutSoccer(ctx context.Context, s Soccer) error {
	s.Email = NormalizeEmail(s.Email)
	teams := s.Teams
	if teams == nil {
		teams = []RememberedTeam{}
	}
	return d.put(ctx, soccerItem{ID: soccerKey(s.Email), Kind: "soccer", Email: s.Email, Teams: teams, Secret: s.Secret})
}

func (d *Dynamo) SoccerLink(ctx context.Context, secret string) (string, error) {
	var it soccerLinkItem
	if err := d.get(ctx, soccerLinkKey(secret), &it); err != nil {
		return "", err
	}
	return it.Email, nil
}

func (d *Dynamo) PutSoccerLink(ctx context.Context, secret, email string) error {
	return d.put(ctx, soccerLinkItem{ID: soccerLinkKey(secret), Kind: "soccerlink", Email: NormalizeEmail(email)})
}

func (d *Dynamo) DeleteSoccerLink(ctx context.Context, secret string) error {
	return d.del(ctx, soccerLinkKey(secret))
}

func (d *Dynamo) GetGoogle(ctx context.Context, email string) (Google, error) {
	var it googleItem
	if err := d.get(ctx, googleKey(email), &it); err != nil {
		return Google{}, err
	}
	return Google{Email: it.Email, Account: it.Account, Token: it.Token, CalendarID: it.CalendarID}, nil
}

func (d *Dynamo) PutGoogle(ctx context.Context, g Google) error {
	g.Email = NormalizeEmail(g.Email)
	return d.put(ctx, googleItem{ID: googleKey(g.Email), Kind: "google", Email: g.Email, Account: g.Account, Token: g.Token, CalendarID: g.CalendarID})
}

func (d *Dynamo) DeleteGoogle(ctx context.Context, email string) error {
	return d.del(ctx, googleKey(email))
}

func (d *Dynamo) GetICloud(ctx context.Context, email string) (ICloud, error) {
	var it icloudItem
	if err := d.get(ctx, icloudKey(email), &it); err != nil {
		return ICloud{}, err
	}
	return it.connection(), nil
}

func (d *Dynamo) PutICloud(ctx context.Context, c ICloud) error {
	c.Email = NormalizeEmail(c.Email)
	return d.put(ctx, icloudItem{ID: icloudKey(c.Email), Kind: "icloud", Email: c.Email, Account: c.Account, Password: c.Password, Home: c.Home, CalendarID: c.CalendarID, Daily: c.Daily, Written: c.Written, Times: c.Times})
}

func (d *Dynamo) DeleteICloud(ctx context.Context, email string) error {
	return d.del(ctx, icloudKey(email))
}

// ListICloud reads the whole table, as ListMembers does. It needn't be
// strongly consistent: the daily update saves through UpdateICloudDaily,
// which checks the item again.
func (d *Dynamo) ListICloud(ctx context.Context) ([]ICloud, error) {
	var out []ICloud
	p := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		FilterExpression:          aws.String("kind = :k"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":k": &types.AttributeValueMemberS{Value: "icloud"}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			// Like get, the error names only the kind of record.
			return nil, fmt.Errorf("list icloud: %w", err)
		}
		var items []icloudItem
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, fmt.Errorf("list icloud: %w", err)
		}
		for _, it := range items {
			out = append(out, it.connection())
		}
	}
	return out, nil
}

// UpdateICloudDaily sets daily, written and times alone, and only on the
// item c was read from: a missing item, one whose password or calendar is no
// longer c's, or one whose daily update has been turned off, fails the
// condition, which is ErrNotFound. The attributes go by placeholder names,
// so none can turn out to be a DynamoDB reserved word.
func (d *Dynamo) UpdateICloudDaily(ctx context.Context, c ICloud) error {
	written, err := attributevalue.Marshal(c.Written)
	if err != nil {
		return err
	}
	times, err := attributevalue.Marshal(c.Times)
	if err != nil {
		return err
	}
	_, err = d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(d.table),
		Key:                 map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: icloudKey(c.Email)}},
		UpdateExpression:    aws.String("SET #daily = :daily, #written = :written, #times = :times"),
		ConditionExpression: aws.String("#password = :password AND #calendar = :calendar AND #daily = :on"),
		ExpressionAttributeNames: map[string]string{
			"#daily": "daily", "#written": "written", "#times": "times", "#password": "password", "#calendar": "calendar_id",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":daily":    &types.AttributeValueMemberBOOL{Value: c.Daily},
			":on":       &types.AttributeValueMemberBOOL{Value: true},
			":written":  written,
			":times":    times,
			":password": &types.AttributeValueMemberS{Value: c.Password},
			":calendar": &types.AttributeValueMemberS{Value: c.CalendarID},
		},
	})
	var changed *types.ConditionalCheckFailedException
	switch {
	case errors.As(err, &changed):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("update icloud: %w", err)
	}
	return nil
}

// IsNotFound reports whether err means the item does not exist.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

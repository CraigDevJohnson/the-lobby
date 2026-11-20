package store

import (
	"context"
	"errors"
	"fmt"
	"strconv"
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
	ID         string   `dynamodbav:"id"`
	Kind       string   `dynamodbav:"kind"`
	Email      string   `dynamodbav:"email"`
	Tools      []string `dynamodbav:"tools"`
	AddedAt    string   `dynamodbav:"added_at"`
	Generation string   `dynamodbav:"generation,omitempty"`
}

type sessionItem struct {
	ID               string `dynamodbav:"id"`
	Kind             string `dynamodbav:"kind"`
	Email            string `dynamodbav:"email"`
	MemberGeneration string `dynamodbav:"member_generation,omitempty"`
	CreatedAt        string `dynamodbav:"created_at"`
	ExpiresAt        int64  `dynamodbav:"expires_at"`
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
	ID           string `dynamodbav:"id"`
	Kind         string `dynamodbav:"kind"`
	Email        string `dynamodbav:"email"`
	Account      string `dynamodbav:"account"`
	Token        string `dynamodbav:"token"`
	CalendarID   string `dynamodbav:"calendar_id"`
	ConnectionID string `dynamodbav:"connection_id,omitempty"`
}

func googleKey(email string) string { return "google#" + NormalizeEmail(email) }

type icloudItem struct {
	ID           string `dynamodbav:"id"`
	Kind         string `dynamodbav:"kind"`
	Email        string `dynamodbav:"email"`
	Account      string `dynamodbav:"account"`
	Password     string `dynamodbav:"password"`
	Home         string `dynamodbav:"home"`
	CalendarID   string `dynamodbav:"calendar_id"`
	ConnectionID string `dynamodbav:"connection_id,omitempty"`
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
	return ICloud{Email: it.Email, Account: it.Account, Password: it.Password, Home: it.Home, CalendarID: it.CalendarID, ConnectionID: it.ConnectionID, Daily: it.Daily, Written: it.Written, Times: it.Times}
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

// connectionCondition identifies the exact credential record a caller
// read. Records created before connection IDs were added remain usable, but
// only while the stored credential still has no ID and is still the same.
func connectionCondition(connectionID, credentialName, credential string) (string, map[string]string, map[string]types.AttributeValue) {
	names := map[string]string{"#connection": "connection_id", "#credential": credentialName}
	values := map[string]types.AttributeValue{":credential": &types.AttributeValueMemberS{Value: credential}}
	if connectionID == "" {
		return "attribute_not_exists(#connection) AND #credential = :credential", names, values
	}
	values[":connection"] = &types.AttributeValueMemberS{Value: connectionID}
	return "#connection = :connection AND #credential = :credential", names, values
}

func conditionalResult(err error, operation string) error {
	var changed *types.ConditionalCheckFailedException
	switch {
	case errors.As(err, &changed):
		return ErrNotFound
	case err != nil:
		return fmt.Errorf("%s: %w", operation, err)
	default:
		return nil
	}
}

func (d *Dynamo) GetMember(ctx context.Context, email string) (Member, error) {
	var it memberItem
	if err := d.get(ctx, memberKey(email), &it); err != nil {
		return Member{}, err
	}
	added, _ := time.Parse(time.RFC3339, it.AddedAt)
	return Member{Email: it.Email, Tools: it.Tools, AddedAt: added, Generation: it.Generation}, nil
}

// CreateMember puts the item only where the table has none for the address.
func (d *Dynamo) CreateMember(ctx context.Context, m Member) error {
	return d.putMember(ctx, m, "attribute_not_exists(#id)", map[string]string{"#id": "id"}, nil)
}

// ReplaceMember puts the item only over one that still has m's generation.
// An item written before generations has no such attribute, and m read from
// it has "", so there the condition is an item without one.
func (d *Dynamo) ReplaceMember(ctx context.Context, m Member) error {
	if m.Generation == "" {
		return d.putMember(ctx, m, "attribute_exists(#id) AND attribute_not_exists(#generation)", map[string]string{"#id": "id", "#generation": "generation"}, nil)
	}
	return d.putMember(ctx, m, "#generation = :generation", map[string]string{"#generation": "generation"},
		map[string]types.AttributeValue{":generation": &types.AttributeValueMemberS{Value: m.Generation}})
}

// putMember writes a Member's item whole, and only while the item in the
// table meets the condition: DynamoDB refusing it is ErrConflict. Like get,
// any other failure names only the kind of record.
func (d *Dynamo) putMember(ctx context.Context, m Member, condition string, names map[string]string, values map[string]types.AttributeValue) error {
	m.Email = NormalizeEmail(m.Email)
	if m.AddedAt.IsZero() {
		m.AddedAt = time.Now().UTC()
	}
	item, err := attributevalue.MarshalMap(memberItem{ID: memberKey(m.Email), Kind: "member", Email: m.Email, Tools: m.Tools, AddedAt: m.AddedAt.Format(time.RFC3339), Generation: m.Generation})
	if err != nil {
		return err
	}
	_, err = d.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                 aws.String(d.table),
		Item:                      item,
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	var changed *types.ConditionalCheckFailedException
	switch {
	case errors.As(err, &changed):
		return ErrConflict
	case err != nil:
		return fmt.Errorf("put member: %w", err)
	}
	return nil
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
			out = append(out, Member{Email: it.Email, Tools: it.Tools, AddedAt: added, Generation: it.Generation})
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
	return Session{ID: id, Email: it.Email, MemberGeneration: it.MemberGeneration, CreatedAt: created, ExpiresAt: exp}, nil
}

func (d *Dynamo) PutSession(ctx context.Context, s Session) error {
	return d.put(ctx, sessionItem{ID: sessionKey(s.ID), Kind: "session", Email: s.Email, MemberGeneration: s.MemberGeneration, CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339), ExpiresAt: s.ExpiresAt.Unix()})
}

// RenewSession advances only the expiry, and only if the item still has the
// expiry read by the request. A deletion makes the condition fail, so an
// in-flight request cannot recreate a deleted session. A later expiry means
// another request already renewed the same live session and is also success.
func (d *Dynamo) RenewSession(ctx context.Context, id string, currentExpiresAt, expiresAt time.Time) error {
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(d.table),
		Key:                 map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: sessionKey(id)}},
		UpdateExpression:    aws.String("SET #expires = :expires"),
		ConditionExpression: aws.String("#expires = :current"),
		ExpressionAttributeNames: map[string]string{
			"#expires": "expires_at",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":current": &types.AttributeValueMemberN{Value: strconv.FormatInt(currentExpiresAt.Unix(), 10)},
			":expires": &types.AttributeValueMemberN{Value: strconv.FormatInt(expiresAt.Unix(), 10)},
		},
	})
	var changed *types.ConditionalCheckFailedException
	switch {
	case errors.As(err, &changed):
		current, getErr := d.GetSession(ctx, id)
		switch {
		case getErr != nil:
			return getErr
		case current.ExpiresAt.After(currentExpiresAt):
			return nil
		default:
			return ErrNotFound
		}
	case err != nil:
		return fmt.Errorf("renew session: %w", err)
	}
	return nil
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
			out = append(out, Session{ID: strings.TrimPrefix(it.ID, "session#"), Email: it.Email, MemberGeneration: it.MemberGeneration, CreatedAt: created, ExpiresAt: exp})
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
	return Google{Email: it.Email, Account: it.Account, Token: it.Token, CalendarID: it.CalendarID, ConnectionID: it.ConnectionID}, nil
}

func (d *Dynamo) PutGoogle(ctx context.Context, g Google) error {
	g.Email = NormalizeEmail(g.Email)
	return d.put(ctx, googleItem{ID: googleKey(g.Email), Kind: "google", Email: g.Email, Account: g.Account, Token: g.Token, CalendarID: g.CalendarID, ConnectionID: g.ConnectionID})
}

func (d *Dynamo) UpdateGoogleCalendar(ctx context.Context, current Google, calendarID string) error {
	condition, names, values := connectionCondition(current.ConnectionID, "token", current.Token)
	condition += " AND #calendar = :current_calendar"
	names["#calendar"] = "calendar_id"
	values[":current_calendar"] = &types.AttributeValueMemberS{Value: current.CalendarID}
	values[":calendar"] = &types.AttributeValueMemberS{Value: calendarID}
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: googleKey(current.Email)}},
		UpdateExpression:          aws.String("SET #calendar = :calendar"),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return conditionalResult(err, "update google")
}

func (d *Dynamo) DeleteGoogleConnection(ctx context.Context, current Google) error {
	condition, names, values := connectionCondition(current.ConnectionID, "token", current.Token)
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: googleKey(current.Email)}},
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return conditionalResult(err, "delete google")
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
	return d.put(ctx, icloudItem{ID: icloudKey(c.Email), Kind: "icloud", Email: c.Email, Account: c.Account, Password: c.Password, Home: c.Home, CalendarID: c.CalendarID, ConnectionID: c.ConnectionID, Daily: c.Daily, Written: c.Written, Times: c.Times})
}

func (d *Dynamo) UpdateICloudCalendar(ctx context.Context, current ICloud, calendarID string) error {
	condition, names, values := connectionCondition(current.ConnectionID, "password", current.Password)
	condition += " AND #calendar = :current_calendar"
	names["#calendar"] = "calendar_id"
	values[":current_calendar"] = &types.AttributeValueMemberS{Value: current.CalendarID}
	values[":calendar"] = &types.AttributeValueMemberS{Value: calendarID}
	update := "SET #calendar = :calendar"
	if current.CalendarID != calendarID {
		written, err := attributevalue.Marshal([]string(nil))
		if err != nil {
			return err
		}
		times, err := attributevalue.Marshal(map[string]GameTime(nil))
		if err != nil {
			return err
		}
		update += ", #written = :written, #times = :times"
		names["#written"], names["#times"] = "written", "times"
		values[":written"], values[":times"] = written, times
	}
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: icloudKey(current.Email)}},
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return conditionalResult(err, "update icloud")
}

func (d *Dynamo) UpdateICloudDailySwitch(ctx context.Context, current ICloud, on bool, calendarID string) error {
	condition, names, values := connectionCondition(current.ConnectionID, "password", current.Password)
	condition += " AND #calendar = :current_calendar"
	if current.Daily {
		condition += " AND #daily = :current_daily"
	} else {
		condition += " AND (attribute_not_exists(#daily) OR #daily = :current_daily)"
	}
	names["#calendar"], names["#daily"] = "calendar_id", "daily"
	values[":current_calendar"] = &types.AttributeValueMemberS{Value: current.CalendarID}
	values[":current_daily"] = &types.AttributeValueMemberBOOL{Value: current.Daily}
	values[":daily"] = &types.AttributeValueMemberBOOL{Value: on}
	update := "SET #daily = :daily"
	if on && current.CalendarID != calendarID {
		written, err := attributevalue.Marshal([]string(nil))
		if err != nil {
			return err
		}
		times, err := attributevalue.Marshal(map[string]GameTime(nil))
		if err != nil {
			return err
		}
		update += ", #calendar = :calendar, #written = :written, #times = :times"
		names["#written"], names["#times"] = "written", "times"
		values[":calendar"] = &types.AttributeValueMemberS{Value: calendarID}
		values[":written"], values[":times"] = written, times
	}
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: icloudKey(current.Email)}},
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return conditionalResult(err, "update icloud")
}

func (d *Dynamo) DeleteICloudConnection(ctx context.Context, current ICloud) error {
	condition, names, values := connectionCondition(current.ConnectionID, "password", current.Password)
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: icloudKey(current.Email)}},
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return conditionalResult(err, "delete icloud")
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
// item c was read from: a missing item, one whose connection ID, password or
// calendar is no longer c's, or one whose daily update has been turned off,
// fails the condition, which is ErrNotFound. The attributes go by placeholder
// names, so none can turn out to be a DynamoDB reserved word.
func (d *Dynamo) UpdateICloudDaily(ctx context.Context, c ICloud) error {
	written, err := attributevalue.Marshal(c.Written)
	if err != nil {
		return err
	}
	times, err := attributevalue.Marshal(c.Times)
	if err != nil {
		return err
	}
	condition, names, values := connectionCondition(c.ConnectionID, "password", c.Password)
	condition += " AND #calendar = :calendar AND #daily = :on"
	names["#daily"], names["#written"], names["#times"], names["#calendar"] = "daily", "written", "times", "calendar_id"
	values[":daily"] = &types.AttributeValueMemberBOOL{Value: c.Daily}
	values[":on"] = &types.AttributeValueMemberBOOL{Value: true}
	values[":written"], values[":times"] = written, times
	values[":calendar"] = &types.AttributeValueMemberS{Value: c.CalendarID}
	_, err = d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: icloudKey(c.Email)}},
		UpdateExpression:          aws.String("SET #daily = :daily, #written = :written, #times = :times"),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	return conditionalResult(err, "update icloud")
}

// IsNotFound reports whether err means the item does not exist.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

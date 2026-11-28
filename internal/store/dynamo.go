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

// Dynamo keeps Members, Sign-in sessions, Members' Let's Plan Soccer
// calendars and the family's wish lists in one DynamoDB table, told apart by
// a prefix on the key. Sessions carry expires_at, which the table's TTL
// setting uses to remove them; nothing else expires. A Member's calendar is
// items of its own, soccer#<email>, soccerlink#<secret>, google#<email> and
// icloud#<email>, separate from member#<email>, which `site member add`
// writes whole. DeleteMember also removes the reverse lookup of any legacy
// Member link. Each wish list is one item, wish#managed#<id> or
// wish#personal#<generation>#<email> (wishKey).
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
	// Removing is the mark DeleteMember sets before it deletes anything. Only
	// DeleteMember writes it, and no item made before it has it.
	Removing bool `dynamodbav:"removing,omitempty"`
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
	Email  string           `dynamodbav:"email"`
	Teams  []RememberedTeam `dynamodbav:"teams"`
	Secret string           `dynamodbav:"secret"`
}

type soccerLinkItem struct {
	Email string `dynamodbav:"email"`
}

type googleItem struct {
	ID            string              `dynamodbav:"id"`
	Kind          string              `dynamodbav:"kind"`
	Email         string              `dynamodbav:"email"`
	Account       string              `dynamodbav:"account"`
	Token         string              `dynamodbav:"token"`
	CalendarID    string              `dynamodbav:"calendar_id"`
	ConnectionID  string              `dynamodbav:"connection_id,omitempty"`
	Daily         bool                `dynamodbav:"daily,omitempty"`
	Written       []string            `dynamodbav:"written,omitempty"`
	Times         map[string]GameTime `dynamodbav:"times,omitempty"`
	DailyRevision int64               `dynamodbav:"daily_revision,omitempty"`
}

func (it googleItem) connection() Google {
	return Google{Email: it.Email, Account: it.Account, Token: it.Token, CalendarID: it.CalendarID, ConnectionID: it.ConnectionID,
		Daily: it.Daily, Written: it.Written, Times: it.Times, DailyRevision: it.DailyRevision}
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

func (d *Dynamo) putForMember(ctx context.Context, email, kind string, item any) error {
	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return err
	}
	return d.writeForMember(ctx, email, "put "+kind, types.TransactWriteItem{Put: &types.Put{TableName: aws.String(d.table), Item: av}})
}

// The site role can Update the Member but cannot perform a ConditionCheck.
// Setting its existing kind lets the transaction require an active Member (ADR 0011).
func (d *Dynamo) writeForMember(ctx context.Context, email, operation string, actions ...types.TransactWriteItem) error {
	member := types.TransactWriteItem{Update: &types.Update{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: memberKey(email)}},
		UpdateExpression:          aws.String("SET #kind = :member"),
		ConditionExpression:       aws.String("attribute_exists(#id) AND attribute_not_exists(#removing)"),
		ExpressionAttributeNames:  map[string]string{"#id": "id", "#kind": "kind", "#removing": "removing"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":member": &types.AttributeValueMemberS{Value: "member"}},
	}}
	_, err := d.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: append([]types.TransactWriteItem{member}, actions...)})
	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) {
		for _, reason := range canceled.CancellationReasons {
			if aws.ToString(reason.Code) == "ConditionalCheckFailed" {
				return ErrNotFound
			}
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
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
	if it.Removing {
		return Member{}, ErrRemoving
	}
	added, _ := time.Parse(time.RFC3339, it.AddedAt)
	return Member{Email: it.Email, Tools: it.Tools, AddedAt: added, Generation: it.Generation}, nil
}

// CreateMember puts the item only where the table has none for the address,
// so never over one being removed.
func (d *Dynamo) CreateMember(ctx context.Context, m Member) error {
	return d.putMember(ctx, m, "attribute_not_exists(#id)", map[string]string{"#id": "id"}, nil)
}

// ReplaceMember puts the item only over one that still has m's generation
// and is not being removed: the put writes the item whole, without the mark,
// so over a marked item it would undo the removal. An item written before
// generations has no such attribute, and m read from it has "", so there the
// condition is an item without one.
func (d *Dynamo) ReplaceMember(ctx context.Context, m Member) error {
	if m.Generation == "" {
		return d.putMember(ctx, m, "attribute_exists(#id) AND attribute_not_exists(#generation) AND attribute_not_exists(#removing)",
			map[string]string{"#id": "id", "#generation": "generation", "#removing": "removing"}, nil)
	}
	return d.putMember(ctx, m, "#generation = :generation AND attribute_not_exists(#removing)", map[string]string{"#generation": "generation", "#removing": "removing"},
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
	// The mark first: from here GetMember answers ErrRemoving, so the Member
	// has no access, and no calendar item of theirs can be written
	// (putForMember), whatever happens to the deletes below. An address with
	// no Member item has nothing to mark, and what is left of its calendar is
	// still deleted.
	// The mark answers the item as marked, so the rest of the removal knows
	// which membership it is removing even if the address is invited again
	// while it runs.
	out, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: memberKey(email)}},
		UpdateExpression:          aws.String("SET #removing = :removing"),
		ConditionExpression:       aws.String("attribute_exists(#id)"),
		ExpressionAttributeNames:  map[string]string{"#id": "id", "#removing": "removing"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":removing": &types.AttributeValueMemberBOOL{Value: true}},
		ReturnValues:              types.ReturnValueAllNew,
	})
	var missing *types.ConditionalCheckFailedException
	marked := err == nil
	var was memberItem
	switch {
	case errors.As(err, &missing):
	case err != nil:
		return fmt.Errorf("mark member: %w", err)
	default:
		if err := attributevalue.UnmarshalMap(out.Attributes, &was); err != nil {
			return fmt.Errorf("mark member: %w", err)
		}
	}
	// Read the calendar before deleting anything because its secret is the
	// Member link's key. The Member itself goes last, so any cleanup failure
	// leaves removal retryable and prevents the same address being added while
	// data from this membership remains.
	s, err := d.GetSoccer(ctx, email)
	switch {
	case errors.Is(err, ErrNotFound):
	case err != nil:
		return err
	default:
		// The link first, so it stops even if the rest fails.
		if s.Secret != "" {
			if err := d.del(ctx, soccerLinkKey(s.Secret)); err != nil {
				return err
			}
		}
		if err := d.del(ctx, soccerKey(email)); err != nil {
			return err
		}
	}
	if err := d.del(ctx, googleKey(email)); err != nil {
		return err
	}
	if err := d.del(ctx, icloudKey(email)); err != nil {
		return err
	}
	// The personal wish list is keyed by the membership marked, so a removal
	// running late never reaches one made after the address was invited
	// again. With no Member item there was no membership to mark, and so no
	// list a removal had not already deleted.
	if !marked {
		return nil
	}
	if err := d.del(ctx, personalWishKey(email, was.Generation)); err != nil {
		return fmt.Errorf("delete wish: %w", err)
	}
	return d.deleteMarkedMember(ctx, email, was.Generation)
}

func (d *Dynamo) deleteMarkedMember(ctx context.Context, email, generation string) error {
	condition := "attribute_exists(#removing) AND attribute_not_exists(#generation)"
	names := map[string]string{"#removing": "removing", "#generation": "generation"}
	var values map[string]types.AttributeValue
	if generation != "" {
		condition = "attribute_exists(#removing) AND #generation = :generation"
		values = map[string]types.AttributeValue{":generation": &types.AttributeValueMemberS{Value: generation}}
	}
	_, err := d.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: memberKey(email)}},
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	if err := conditionalResult(err, "delete member"); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
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

func (d *Dynamo) UpdateSoccerTeams(ctx context.Context, was Soccer, teams []RememberedTeam) error {
	if was.Teams == nil {
		was.Teams = []RememberedTeam{}
	}
	if teams == nil {
		teams = []RememberedTeam{}
	}
	previous, err := attributevalue.Marshal(was.Teams)
	if err != nil {
		return err
	}
	next, err := attributevalue.Marshal(teams)
	if err != nil {
		return err
	}
	names := map[string]string{"#teams": "teams", "#secret": "secret"}
	values := map[string]types.AttributeValue{
		":next": next, ":previous": previous, ":secret": &types.AttributeValueMemberS{Value: was.Secret},
	}
	condition := "#teams = :previous AND #secret = :secret"
	update := "SET #teams = :next"
	if was.Secret == "" {
		condition = "#teams = :previous AND (attribute_not_exists(#secret) OR #secret = :secret)"
		if len(was.Teams) == 0 {
			condition = "attribute_not_exists(#id) OR (" + condition + ")"
			update += ", #kind = :kind, #email = :email"
			names["#id"], names["#kind"], names["#email"] = "id", "kind", "email"
			values[":kind"] = &types.AttributeValueMemberS{Value: "soccer"}
			values[":email"] = &types.AttributeValueMemberS{Value: NormalizeEmail(was.Email)}
		}
	}
	return d.writeForMember(ctx, was.Email, "update soccer teams", types.TransactWriteItem{Update: &types.Update{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: soccerKey(was.Email)}},
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	}})
}

func (d *Dynamo) RevokeSoccerLink(ctx context.Context, was Soccer) error {
	if was.Secret == "" {
		return nil
	}
	return d.writeForMember(ctx, was.Email, "revoke soccer link",
		types.TransactWriteItem{Update: &types.Update{
			TableName:                 aws.String(d.table),
			Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: soccerKey(was.Email)}},
			UpdateExpression:          aws.String("REMOVE #secret"),
			ConditionExpression:       aws.String("#secret = :secret"),
			ExpressionAttributeNames:  map[string]string{"#secret": "secret"},
			ExpressionAttributeValues: map[string]types.AttributeValue{":secret": &types.AttributeValueMemberS{Value: was.Secret}},
		}},
		types.TransactWriteItem{Delete: &types.Delete{
			TableName: aws.String(d.table),
			Key:       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: soccerLinkKey(was.Secret)}},
		}},
	)
}

func (d *Dynamo) SoccerLink(ctx context.Context, secret string) (string, error) {
	var it soccerLinkItem
	if err := d.get(ctx, soccerLinkKey(secret), &it); err != nil {
		return "", err
	}
	return it.Email, nil
}

func (d *Dynamo) GetGoogle(ctx context.Context, email string) (Google, error) {
	var it googleItem
	if err := d.get(ctx, googleKey(email), &it); err != nil {
		return Google{}, err
	}
	return it.connection(), nil
}

func (d *Dynamo) PutGoogle(ctx context.Context, g Google) error {
	g.Email = NormalizeEmail(g.Email)
	return d.putForMember(ctx, g.Email, "google", googleItem{ID: googleKey(g.Email), Kind: "google", Email: g.Email, Account: g.Account, Token: g.Token, CalendarID: g.CalendarID, ConnectionID: g.ConnectionID,
		Daily: g.Daily, Written: g.Written, Times: g.Times, DailyRevision: g.DailyRevision})
}

func (d *Dynamo) UpdateGoogleCalendar(ctx context.Context, current Google, calendarID string) error {
	return d.updateCalendar(ctx, current.record(), calendarID, nil)
}

func (d *Dynamo) UpdateGoogleDailySwitch(ctx context.Context, current Google, on bool, calendarID string) error {
	return d.updateCalendar(ctx, current.record(), calendarID, &on)
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
	return d.putForMember(ctx, c.Email, "icloud", icloudItem{ID: icloudKey(c.Email), Kind: "icloud", Email: c.Email, Account: c.Account, Password: c.Password, Home: c.Home, CalendarID: c.CalendarID, ConnectionID: c.ConnectionID, Daily: c.Daily, Written: c.Written, Times: c.Times})
}

func (d *Dynamo) UpdateICloudCalendar(ctx context.Context, current ICloud, calendarID string) error {
	return d.updateCalendar(ctx, current.record(), calendarID, nil)
}

func (d *Dynamo) UpdateICloudDailySwitch(ctx context.Context, current ICloud, on bool, calendarID string) error {
	return d.updateCalendar(ctx, current.record(), calendarID, &on)
}

type calendarRecord struct {
	key, connectionID, credentialName, credential string
	calendarID                                    string
	daily                                         bool
	dailyRevision                                 *int64
}

func (g Google) record() calendarRecord {
	return calendarRecord{key: googleKey(g.Email), connectionID: g.ConnectionID, credentialName: "token", credential: g.Token,
		calendarID: g.CalendarID, daily: g.Daily, dailyRevision: &g.DailyRevision}
}

func (c ICloud) record() calendarRecord {
	return calendarRecord{key: icloudKey(c.Email), connectionID: c.ConnectionID, credentialName: "password", credential: c.Password,
		calendarID: c.CalendarID, daily: c.Daily}
}

func (d *Dynamo) updateCalendar(ctx context.Context, r calendarRecord, calendarID string, daily *bool) error {
	condition, names, values := connectionCondition(r.connectionID, r.credentialName, r.credential)
	condition += " AND #calendar = :current_calendar"
	names["#calendar"] = "calendar_id"
	values[":current_calendar"] = &types.AttributeValueMemberS{Value: r.calendarID}
	if r.daily {
		condition += " AND #daily = :current_daily"
	} else {
		condition += " AND (attribute_not_exists(#daily) OR #daily = :current_daily)"
	}
	names["#daily"] = "daily"
	values[":current_daily"] = &types.AttributeValueMemberBOOL{Value: r.daily}
	// A one-time copy can use another calendar without redirecting the
	// Member's automatic updates. Still check the live connection.
	if daily == nil && r.daily {
		calendarID = r.calendarID
	}
	var set []string
	if daily != nil {
		values[":daily"] = &types.AttributeValueMemberBOOL{Value: *daily}
		set = append(set, "#daily = :daily")
		if !*daily {
			calendarID = r.calendarID
		}
	}
	if daily == nil || r.calendarID != calendarID {
		set = append(set, "#calendar = :calendar")
		values[":calendar"] = &types.AttributeValueMemberS{Value: calendarID}
	}
	if r.calendarID != calendarID {
		set = append(set, "#written = :written", "#times = :times")
		names["#written"], names["#times"] = "written", "times"
		values[":written"] = &types.AttributeValueMemberNULL{Value: true}
		values[":times"] = &types.AttributeValueMemberNULL{Value: true}
	}
	update := "SET " + strings.Join(set, ", ")
	if r.dailyRevision != nil && (r.calendarID != calendarID || (daily != nil && *daily != r.daily)) {
		update += " ADD #revision :one"
		names["#revision"] = "daily_revision"
		values[":one"] = &types.AttributeValueMemberN{Value: "1"}
	}
	return d.updateConnection(ctx, r.key, update, condition, names, values)
}

func (d *Dynamo) updateConnection(ctx context.Context, key, update, condition string, names map[string]string, values map[string]types.AttributeValue) error {
	_, err := d.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: key}},
		UpdateExpression:          aws.String(update),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	})
	kind, _, _ := strings.Cut(key, "#")
	return conditionalResult(err, "update "+kind)
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

func (d *Dynamo) ListGoogle(ctx context.Context) ([]Google, error) {
	return listConnections[googleItem, Google](ctx, d, "google")
}

func (d *Dynamo) ListICloud(ctx context.Context) ([]ICloud, error) {
	return listConnections[icloudItem, ICloud](ctx, d, "icloud")
}

// Scheduled workers check each connection again before writing to its calendar.
func listConnections[T interface{ connection() C }, C any](ctx context.Context, d *Dynamo, kind string) ([]C, error) {
	var out []C
	p := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		FilterExpression:          aws.String("kind = :k"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":k": &types.AttributeValueMemberS{Value: kind}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list %s: %w", kind, err)
		}
		var items []T
		if err := attributevalue.UnmarshalListOfMaps(page.Items, &items); err != nil {
			return nil, fmt.Errorf("list %s: %w", kind, err)
		}
		for _, it := range items {
			out = append(out, it.connection())
		}
	}
	return out, nil
}

func (d *Dynamo) UpdateGoogleDaily(ctx context.Context, g Google) error {
	return d.updateDaily(ctx, g.record(), g.Written, g.Times)
}

func (d *Dynamo) UpdateICloudDaily(ctx context.Context, c ICloud) error {
	return d.updateDaily(ctx, c.record(), c.Written, c.Times)
}

func (d *Dynamo) updateDaily(ctx context.Context, r calendarRecord, ids []string, gameTimes map[string]GameTime) error {
	written, err := attributevalue.Marshal(ids)
	if err != nil {
		return err
	}
	times, err := attributevalue.Marshal(gameTimes)
	if err != nil {
		return err
	}
	condition, names, values := connectionCondition(r.connectionID, r.credentialName, r.credential)
	condition += " AND #calendar = :calendar AND #daily = :on"
	names["#daily"], names["#written"], names["#times"], names["#calendar"] = "daily", "written", "times", "calendar_id"
	values[":daily"] = &types.AttributeValueMemberBOOL{Value: r.daily}
	values[":on"] = &types.AttributeValueMemberBOOL{Value: true}
	values[":written"], values[":times"] = written, times
	values[":calendar"] = &types.AttributeValueMemberS{Value: r.calendarID}
	update := "SET #daily = :daily, #written = :written, #times = :times"
	if r.dailyRevision != nil {
		names["#revision"] = "daily_revision"
		value := *r.dailyRevision
		values[":revision"] = &types.AttributeValueMemberN{Value: strconv.FormatInt(value, 10)}
		if value == 0 {
			condition += " AND (attribute_not_exists(#revision) OR #revision = :revision)"
		} else {
			condition += " AND #revision = :revision"
		}
		update += " ADD #revision :one"
		values[":one"] = &types.AttributeValueMemberN{Value: "1"}
	}
	return d.updateConnection(ctx, r.key, update, condition, names, values)
}

// IsNotFound reports whether err means the item does not exist.
func IsNotFound(err error) bool { return errors.Is(err, ErrNotFound) }

// wishItem is one wish list's item. Owner and OwnerGeneration are a
// personal list's membership; a legacy membership's generation is "", and
// is left out.
type wishItem struct {
	ID              string `dynamodbav:"id"`
	Kind            string `dynamodbav:"kind"`
	ListID          string `dynamodbav:"list_id"`
	ListKind        string `dynamodbav:"list_kind"`
	Name            string `dynamodbav:"name"`
	Revision        int64  `dynamodbav:"revision"`
	Wishes          []Wish `dynamodbav:"wishes"`
	Owner           string `dynamodbav:"owner,omitempty"`
	OwnerGeneration string `dynamodbav:"owner_generation,omitempty"`
}

func (it wishItem) list() WishList {
	return WishList{ID: it.ListID, Kind: it.ListKind, Name: it.Name, Revision: it.Revision, Wishes: it.Wishes, Owner: it.Owner, OwnerGeneration: it.OwnerGeneration}
}

// ListWishLists reads the Members with the lists in one strongly consistent
// scan, so a personal list shows only beside the live membership it was made
// under: never one whose Member is being removed, or one left from an
// earlier invitation of the address.
func (d *Dynamo) ListWishLists(ctx context.Context) ([]WishList, error) {
	type membership struct{ generation string }
	live := map[string]membership{}
	var lists []WishList
	p := dynamodb.NewScanPaginator(d.client, &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		ConsistentRead:            aws.Bool(true),
		FilterExpression:          aws.String("#kind = :member OR #kind = :wish"),
		ExpressionAttributeNames:  map[string]string{"#kind": "kind"},
		ExpressionAttributeValues: map[string]types.AttributeValue{":member": &types.AttributeValueMemberS{Value: "member"}, ":wish": &types.AttributeValueMemberS{Value: "wish"}},
	})
	for p.HasMorePages() {
		page, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list wish: %w", err)
		}
		for _, raw := range page.Items {
			kind, _ := raw["kind"].(*types.AttributeValueMemberS)
			switch {
			case kind == nil:
			case kind.Value == "member":
				var it memberItem
				if err := attributevalue.UnmarshalMap(raw, &it); err != nil {
					return nil, fmt.Errorf("list wish: %w", err)
				}
				if !it.Removing {
					live[NormalizeEmail(it.Email)] = membership{it.Generation}
				}
			case kind.Value == "wish":
				var it wishItem
				if err := attributevalue.UnmarshalMap(raw, &it); err != nil {
					return nil, fmt.Errorf("list wish: %w", err)
				}
				lists = append(lists, it.list())
			}
		}
	}
	out := make([]WishList, 0, len(lists))
	for _, l := range lists {
		if l.Kind == WishPersonal {
			m, ok := live[NormalizeEmail(l.Owner)]
			if !ok || m.generation != l.OwnerGeneration {
				continue
			}
		}
		out = append(out, l)
	}
	return out, nil
}

func (d *Dynamo) SaveWishList(ctx context.Context, by WishEditor, l WishList) (WishList, error) {
	key, err := wishKey(l.Kind, l.ID, by.Email, by.Generation)
	if err != nil {
		return WishList{}, err
	}
	saved := l.saved(by)
	item, err := attributevalue.MarshalMap(wishItem{ID: key, Kind: "wish", ListID: saved.ID, ListKind: saved.Kind, Name: saved.Name, Revision: saved.Revision,
		Wishes: saved.Wishes, Owner: saved.Owner, OwnerGeneration: saved.OwnerGeneration})
	if err != nil {
		return WishList{}, err
	}
	put := &types.Put{TableName: aws.String(d.table), Item: item}
	put.ConditionExpression, put.ExpressionAttributeNames, put.ExpressionAttributeValues = wishCondition(l)
	if err := d.writeAsEditor(ctx, by, "save wish", types.TransactWriteItem{Put: put}); err != nil {
		return WishList{}, err
	}
	return saved, nil
}

func (d *Dynamo) DeleteWishList(ctx context.Context, by WishEditor, l WishList) error {
	key, err := wishKey(l.Kind, l.ID, by.Email, by.Generation)
	if err != nil {
		return err
	}
	if l.Revision < 1 {
		// No list is ever saved at revision 0.
		return ErrConflict
	}
	del := &types.Delete{TableName: aws.String(d.table), Key: map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: key}}}
	del.ConditionExpression, del.ExpressionAttributeNames, del.ExpressionAttributeValues = wishCondition(l)
	return d.writeAsEditor(ctx, by, "delete wish", types.TransactWriteItem{Delete: del})
}

// wishCondition holds a write to the list the editor read: none at all for
// a create, otherwise the same list at the same revision.
func wishCondition(l WishList) (*string, map[string]string, map[string]types.AttributeValue) {
	if l.Revision == 0 {
		return aws.String("attribute_not_exists(#id)"), map[string]string{"#id": "id"}, nil
	}
	return aws.String("#list = :list AND #revision = :revision"), map[string]string{"#list": "list_id", "#revision": "revision"},
		map[string]types.AttributeValue{
			":list":     &types.AttributeValueMemberS{Value: l.ID},
			":revision": &types.AttributeValueMemberN{Value: strconv.FormatInt(l.Revision, 10)},
		}
}

// writeAsEditor applies a wish list write in one transaction with a
// condition on the editor's Member item: still the membership the request
// checked, not being removed, and holding every grant the change needs. As
// in writeForMember, the condition rides on an Update that sets the item's
// kind to what it is, since the site role cannot perform a ConditionCheck.
// DynamoDB says why it cancelled for each action in order: the Member's
// condition refused is ErrAccessChanged, and the list's is ErrConflict.
func (d *Dynamo) writeAsEditor(ctx context.Context, by WishEditor, operation string, write types.TransactWriteItem) error {
	names := map[string]string{"#kind": "kind", "#generation": "generation", "#removing": "removing"}
	values := map[string]types.AttributeValue{":member": &types.AttributeValueMemberS{Value: "member"}}
	condition := "#generation = :generation AND attribute_not_exists(#removing)"
	if by.Generation == "" {
		names["#id"] = "id"
		condition = "attribute_exists(#id) AND attribute_not_exists(#generation) AND attribute_not_exists(#removing)"
	} else {
		values[":generation"] = &types.AttributeValueMemberS{Value: by.Generation}
	}
	if len(by.Need) > 0 {
		names["#tools"] = "tools"
	}
	for i, g := range by.Need {
		v := ":grant" + strconv.Itoa(i)
		condition += " AND contains(#tools, " + v + ")"
		values[v] = &types.AttributeValueMemberS{Value: g}
	}
	member := types.TransactWriteItem{Update: &types.Update{
		TableName:                 aws.String(d.table),
		Key:                       map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: memberKey(by.Email)}},
		UpdateExpression:          aws.String("SET #kind = :member"),
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	}}
	_, err := d.client.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: []types.TransactWriteItem{member, write}})
	var canceled *types.TransactionCanceledException
	if errors.As(err, &canceled) {
		reasons := canceled.CancellationReasons
		switch {
		case len(reasons) > 0 && aws.ToString(reasons[0].Code) == "ConditionalCheckFailed":
			return ErrAccessChanged
		case len(reasons) > 1 && aws.ToString(reasons[1].Code) == "ConditionalCheckFailed":
			return ErrConflict
		}
	}
	if err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

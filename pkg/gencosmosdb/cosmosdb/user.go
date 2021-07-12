package cosmosdb

import (
	"context"
	"net/http"
)

// User represents a user
type User struct {
	ID          string `json:"id,omitempty"`
	ResourceID  string `json:"_rid,omitempty"`
	Timestamp   int    `json:"_ts,omitempty"`
	Self        string `json:"_self,omitempty"`
	ETag        string `json:"_etag,omitempty"`
	Permissions string `json:"_permissions,omitempty"`
}

// Users represents users
type Users struct {
	Count      int     `json:"_count,omitempty"`
	ResourceID string  `json:"_rid,omitempty"`
	Users      []*User `json:"Users,omitempty"`
}

type userClient struct {
	*databaseClient
	path string
}

// UserClient is a user client
type UserClient interface {
	Create(context.Context, *User) (*User, error)
	List() UserIterator
	ListAll(context.Context) (*Users, error)
	Get(context.Context, string) (*User, error)
	Delete(context.Context, *User) error
	Replace(context.Context, *User) (*User, error)
}

type userListIterator struct {
	*userClient
	continuation string
	done         bool
}

// UserIterator is a user iterator
type UserIterator interface {
	Next(context.Context) (*Users, error)
}

// NewUserClient returns a new user client
func NewUserClient(c DatabaseClient, dbid string) UserClient {
	return &userClient{
		databaseClient: c.(*databaseClient),
		path:           "dbs/" + dbid,
	}
}

func (c *userClient) all(ctx context.Context, i UserIterator) (*Users, error) {
	allusers := &Users{}

	for {
		users, err := i.Next(ctx)
		if err != nil {
			return nil, err
		}
		if users == nil {
			break
		}

		allusers.Count += users.Count
		allusers.ResourceID = users.ResourceID
		allusers.Users = append(allusers.Users, users.Users...)
	}

	return allusers, nil
}

func (c *userClient) Create(ctx context.Context, newuser *User) (user *User, err error) {
	err = c.do(ctx, http.MethodPost, c.path+"/users", "users", c.path, http.StatusCreated, &newuser, &user, nil)
	return
}

func (c *userClient) List() UserIterator {
	return &userListIterator{userClient: c}
}

func (c *userClient) ListAll(ctx context.Context) (*Users, error) {
	return c.all(ctx, c.List())
}

func (c *userClient) Get(ctx context.Context, userid string) (user *User, err error) {
	err = c.do(ctx, http.MethodGet, c.path+"/users/"+userid, "users", c.path+"/users/"+userid, http.StatusOK, nil, &user, nil)
	return
}

func (c *userClient) Delete(ctx context.Context, user *User) error {
	if user.ETag == "" {
		return ErrETagRequired
	}
	headers := http.Header{}
	headers.Set("If-Match", user.ETag)
	return c.do(ctx, http.MethodDelete, c.path+"/users/"+user.ID, "users", c.path+"/users/"+user.ID, http.StatusNoContent, nil, nil, headers)
}

func (c *userClient) Replace(ctx context.Context, newuser *User) (user *User, err error) {
	err = c.do(ctx, http.MethodPost, c.path+"/users/"+newuser.ID, "users", c.path+"/users/"+newuser.ID, http.StatusCreated, &newuser, &user, nil)
	return
}

func (i *userListIterator) Next(ctx context.Context) (users *Users, err error) {
	if i.done {
		return
	}

	headers := http.Header{}
	if i.continuation != "" {
		headers.Set("X-Ms-Continuation", i.continuation)
	}

	err = i.do(ctx, http.MethodGet, i.path+"/users", "users", i.path, http.StatusOK, nil, &users, headers)
	if err != nil {
		return
	}

	i.continuation = headers.Get("X-Ms-Continuation")
	i.done = i.continuation == ""

	return
}

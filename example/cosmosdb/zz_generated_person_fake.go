// Code generated by github.com/jim-minter/go-cosmosdb, DO NOT EDIT.

package cosmosdb

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"

	"github.com/ugorji/go/codec"

	pkg "github.com/jim-minter/go-cosmosdb/example/types"
)

type fakePersonTrigger func(context.Context, *pkg.Person) error
type fakePersonQuery func(PersonClient, *Query, *Options) PersonRawIterator

var _ PersonClient = &FakePersonClient{}

func NewFakePersonClient(h *codec.JsonHandle) *FakePersonClient {
	return &FakePersonClient{
		docs:              make(map[string][]byte),
		triggers:          make(map[string]fakePersonTrigger),
		queries:           make(map[string]fakePersonQuery),
		jsonHandle:        h,
		lock:              &sync.RWMutex{},
		sorter:            func(in []*pkg.Person) {},
		checkDocsConflict: func(*pkg.Person, *pkg.Person) bool { return false },
	}
}

type FakePersonClient struct {
	docs       map[string][]byte
	jsonHandle *codec.JsonHandle
	lock       *sync.RWMutex
	triggers   map[string]fakePersonTrigger
	queries    map[string]fakePersonQuery
	sorter     func([]*pkg.Person)

	// returns true if documents conflict
	checkDocsConflict func(*pkg.Person, *pkg.Person) bool

	// unavailable, if not nil, is an error to throw when attempting to
	// communicate with this Client
	unavailable error
}

func (c *FakePersonClient) decodePerson(s []byte) (res *pkg.Person, err error) {
	err = codec.NewDecoderBytes(s, c.jsonHandle).Decode(&res)
	return
}

func (c *FakePersonClient) encodePerson(doc *pkg.Person) (res []byte, err error) {
	err = codec.NewEncoderBytes(&res, c.jsonHandle).Encode(doc)
	return
}

func (c *FakePersonClient) MakeUnavailable(err error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.unavailable = err
}

func (c *FakePersonClient) UseSorter(sorter func([]*pkg.Person)) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.sorter = sorter
}

func (c *FakePersonClient) UseDocumentConflictChecker(checker func(*pkg.Person, *pkg.Person) bool) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.checkDocsConflict = checker
}

func (c *FakePersonClient) InjectTrigger(trigger string, impl fakePersonTrigger) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.triggers[trigger] = impl
}

func (c *FakePersonClient) InjectQuery(query string, impl fakePersonQuery) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.queries[query] = impl
}

func (c *FakePersonClient) encodeAndCopy(doc *pkg.Person) (*pkg.Person, []byte, error) {
	encoded, err := c.encodePerson(doc)
	if err != nil {
		return nil, nil, err
	}
	res, err := c.decodePerson(encoded)
	if err != nil {
		return nil, nil, err
	}
	return res, encoded, err
}

func (c *FakePersonClient) apply(ctx context.Context, partitionkey string, doc *pkg.Person, options *Options, isNew bool) (*pkg.Person, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.unavailable != nil {
		return nil, c.unavailable
	}

	var docExists bool

	if options != nil {
		err := c.processPreTriggers(ctx, doc, options)
		if err != nil {
			return nil, err
		}
	}

	res, enc, err := c.encodeAndCopy(doc)
	if err != nil {
		return nil, err
	}

	for _, ext := range c.docs {
		dec, err := c.decodePerson(ext)
		if err != nil {
			return nil, err
		}

		if dec.ID == res.ID {
			// If the document exists in the database, we want to error out in a
			// create but mark the document as extant so it can be replaced if
			// it is an update
			if isNew {
				return nil, &Error{
					StatusCode: http.StatusConflict,
					Message:    "Entity with the specified id already exists in the system",
				}
			} else {
				docExists = true
			}
		} else {
			if c.checkDocsConflict(dec, res) {
				return nil, &Error{
					StatusCode: http.StatusConflict,
					Message:    "Entity with the specified id already exists in the system",
				}
			}
		}
	}

	if !isNew && !docExists {
		return nil, &Error{StatusCode: http.StatusNotFound}
	}

	c.docs[doc.ID] = enc
	return res, nil
}

func (c *FakePersonClient) Create(ctx context.Context, partitionkey string, doc *pkg.Person, options *Options) (*pkg.Person, error) {
	return c.apply(ctx, partitionkey, doc, options, true)
}

func (c *FakePersonClient) Replace(ctx context.Context, partitionkey string, doc *pkg.Person, options *Options) (*pkg.Person, error) {
	return c.apply(ctx, partitionkey, doc, options, false)
}

func (c *FakePersonClient) List(*Options) PersonIterator {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.unavailable != nil {
		return NewFakePersonClientErroringRawIterator(c.unavailable)
	}

	docs := make([]*pkg.Person, 0, len(c.docs))
	for _, d := range c.docs {
		r, err := c.decodePerson(d)
		if err != nil {
			return NewFakePersonClientErroringRawIterator(err)
		}
		docs = append(docs, r)
	}
	c.sorter(docs)
	return NewFakePersonClientRawIterator(docs, 0)
}

func (c *FakePersonClient) ListAll(ctx context.Context, opts *Options) (*pkg.People, error) {
	iter := c.List(opts)
	people, err := iter.Next(ctx, -1)
	if err != nil {
		return nil, err
	}
	return people, nil
}

func (c *FakePersonClient) Get(ctx context.Context, partitionkey string, documentId string, options *Options) (*pkg.Person, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.unavailable != nil {
		return nil, c.unavailable
	}

	out, ext := c.docs[documentId]
	if !ext {
		return nil, &Error{StatusCode: http.StatusNotFound}
	}
	return c.decodePerson(out)
}

func (c *FakePersonClient) Delete(ctx context.Context, partitionKey string, doc *pkg.Person, options *Options) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.unavailable != nil {
		return c.unavailable
	}

	_, ext := c.docs[doc.ID]
	if !ext {
		return &Error{StatusCode: http.StatusNotFound}
	}

	delete(c.docs, doc.ID)
	return nil
}

func (c *FakePersonClient) ChangeFeed(*Options) PersonIterator {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.unavailable != nil {
		return NewFakePersonClientErroringRawIterator(c.unavailable)
	}
	return NewFakePersonClientErroringRawIterator(ErrNotImplemented)
}

func (c *FakePersonClient) processPreTriggers(ctx context.Context, doc *pkg.Person, options *Options) error {
	for _, trigger := range options.PreTriggers {
		trig, ok := c.triggers[trigger]
		if ok {
			err := trig(ctx, doc)
			if err != nil {
				return err
			}
		} else {
			return ErrNotImplemented
		}
	}
	return nil
}

func (c *FakePersonClient) Query(name string, query *Query, options *Options) PersonRawIterator {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if c.unavailable != nil {
		return NewFakePersonClientErroringRawIterator(c.unavailable)
	}

	quer, ok := c.queries[query.Query]
	if ok {
		return quer(c, query, options)
	} else {
		return NewFakePersonClientErroringRawIterator(ErrNotImplemented)
	}
}

func (c *FakePersonClient) QueryAll(ctx context.Context, partitionkey string, query *Query, options *Options) (*pkg.People, error) {
	iter := c.Query("", query, options)
	return iter.Next(ctx, -1)
}

// NewFakePersonClientRawIterator creates a RawIterator that will produce only
// People from Next() and NextRaw().
func NewFakePersonClientRawIterator(docs []*pkg.Person, continuation int) PersonRawIterator {
	return &fakePersonClientRawIterator{docs: docs, continuation: continuation}
}

type fakePersonClientRawIterator struct {
	docs         []*pkg.Person
	continuation int
	done         bool
}

func (i *fakePersonClientRawIterator) Next(ctx context.Context, maxItemCount int) (out *pkg.People, err error) {
	err = i.NextRaw(ctx, maxItemCount, &out)
	return
}

func (i *fakePersonClientRawIterator) NextRaw(ctx context.Context, maxItemCount int, out interface{}) error {
	if i.done {
		return nil
	}

	var docs []*pkg.Person
	if maxItemCount == -1 {
		docs = i.docs[i.continuation:]
		i.continuation = len(i.docs)
		i.done = true
	} else {
		max := i.continuation + maxItemCount
		if max > len(i.docs) {
			max = len(i.docs)
		}
		docs = i.docs[i.continuation:max]
		i.continuation += max
		i.done = i.Continuation() == ""
	}

	y := reflect.ValueOf(out)
	d := &pkg.People{}
	d.People = docs
	d.Count = len(d.People)
	y.Elem().Set(reflect.ValueOf(d))
	return nil
}

func (i *fakePersonClientRawIterator) Continuation() string {
	if i.continuation >= len(i.docs) {
		return ""
	}
	return fmt.Sprintf("%d", i.continuation)
}

// fakePersonErroringRawIterator is a RawIterator that will return an error on use.
func NewFakePersonClientErroringRawIterator(err error) *fakePersonErroringRawIterator {
	return &fakePersonErroringRawIterator{err: err}
}

type fakePersonErroringRawIterator struct {
	err error
}

func (i *fakePersonErroringRawIterator) Next(ctx context.Context, maxItemCount int) (*pkg.People, error) {
	return nil, i.err
}

func (i *fakePersonErroringRawIterator) NextRaw(context.Context, int, interface{}) error {
	return i.err
}

func (i *fakePersonErroringRawIterator) Continuation() string {
	return ""
}

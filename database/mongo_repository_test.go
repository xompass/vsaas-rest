package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-errors/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Test model for testing
type TestModel struct {
	ID       string    `bson:"_id,omitempty" json:"id"`
	Name     string    `bson:"name" json:"name"`
	Email    string    `bson:"email" json:"email"`
	Age      int       `bson:"age" json:"age"`
	Created  time.Time `bson:"created" json:"created"`
	Modified time.Time `bson:"modified" json:"modified"`
	Deleted  time.Time `bson:"deleted,omitempty" json:"deleted,omitempty"`
}

func (t *TestModel) GetTableName() string {
	return "test_models"
}

func (t *TestModel) GetModelName() string {
	return "TestModel"
}

func (t *TestModel) BeforeCreate() error {
	t.Created = time.Now()
	t.Modified = time.Now()
	return nil
}

// Mock MongoDB Collection
type MockMongoCollection struct {
	mock.Mock
	documents []any
}

func (m *MockMongoCollection) Find(ctx context.Context, filter any, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).(*mongo.Cursor), args.Error(1)
}

func (m *MockMongoCollection) FindOne(ctx context.Context, filter any, opts ...*options.FindOneOptions) *mongo.SingleResult {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).(*mongo.SingleResult)
}

func (m *MockMongoCollection) InsertOne(ctx context.Context, document any, opts ...*options.InsertOneOptions) (*mongo.InsertOneResult, error) {
	args := m.Called(ctx, document, opts)
	return args.Get(0).(*mongo.InsertOneResult), args.Error(1)
}

func (m *MockMongoCollection) UpdateOne(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateOneOptions]) (*mongo.UpdateResult, error) {
	args := m.Called(ctx, filter, update, opts)
	return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}

func (m *MockMongoCollection) UpdateMany(ctx context.Context, filter any, update any, opts ...options.Lister[options.UpdateManyOptions]) (*mongo.UpdateResult, error) {
	args := m.Called(ctx, filter, update, opts)
	return args.Get(0).(*mongo.UpdateResult), args.Error(1)
}

func (m *MockMongoCollection) DeleteOne(ctx context.Context, filter any, opts ...options.Lister[options.DeleteOneOptions]) (*mongo.DeleteResult, error) {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).(*mongo.DeleteResult), args.Error(1)
}

func (m *MockMongoCollection) DeleteMany(ctx context.Context, filter any, opts ...options.Lister[options.DeleteManyOptions]) (*mongo.DeleteResult, error) {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).(*mongo.DeleteResult), args.Error(1)
}

func (m *MockMongoCollection) CountDocuments(ctx context.Context, filter any, opts ...*options.CountOptions) (int64, error) {
	args := m.Called(ctx, filter, opts)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockMongoCollection) FindOneAndUpdate(ctx context.Context, filter any, update any, opts ...*options.FindOneAndUpdateOptions) *mongo.SingleResult {
	args := m.Called(ctx, filter, update, opts)
	return args.Get(0).(*mongo.SingleResult)
}

// Mock Repository for maintaining logic without DB operations
type MockMongoRepository struct {
	documents map[string]*TestModel
	nextID    int
}

func NewMockMongoRepository() *MockMongoRepository {
	return &MockMongoRepository{
		documents: make(map[string]*TestModel),
		nextID:    1,
	}
}

func (m *MockMongoRepository) Find(ctx context.Context, filterBuilder *FilterBuilder) ([]*TestModel, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	var results []*TestModel
	for _, doc := range m.documents {
		// Simple mock logic - in real mock you'd parse the filter
		results = append(results, doc)
	}

	// Apply limit if specified
	if filterBuilder.limit != nil && len(results) > int(*filterBuilder.limit) {
		results = results[:*filterBuilder.limit]
	}

	if results == nil {
		return []*TestModel{}, nil
	}
	return results, nil
}

func (m *MockMongoRepository) FindOne(ctx context.Context, filterBuilder *FilterBuilder) (*TestModel, error) {
	if filterBuilder == nil {
		filterBuilder = NewFilter()
	}

	// Simple mock - return first document or nil
	for _, doc := range m.documents {
		return doc, nil
	}
	return nil, nil
}

func (m *MockMongoRepository) FindById(ctx context.Context, id any, filterBuilder *FilterBuilder) (*TestModel, error) {
	if id == nil {
		return nil, errors.New("id cannot be nil")
	}

	idStr, ok := id.(string)
	if !ok {
		return nil, errors.New("id must be string")
	}

	doc, exists := m.documents[idStr]
	if !exists {
		return nil, nil
	}
	return doc, nil
}

func (m *MockMongoRepository) Insert(ctx context.Context, doc *TestModel) (any, error) {
	if hook, ok := any(doc).(BeforeCreateHook); ok {
		if err := hook.BeforeCreate(); err != nil {
			return nil, err
		}
	}

	id := fmt.Sprintf("id_%d", m.nextID)
	m.nextID++
	doc.ID = id
	m.documents[id] = doc
	return id, nil
}

func (m *MockMongoRepository) Create(ctx context.Context, doc *TestModel) (*TestModel, error) {
	insertedID, err := m.Insert(ctx, doc)
	if err != nil {
		return nil, err
	}
	return m.FindById(ctx, insertedID, NewFilter())
}

func (m *MockMongoRepository) UpdateById(ctx context.Context, id any, update any) error {
	if id == nil {
		return errors.New("id cannot be nil")
	}
	if update == nil {
		return errors.New("update cannot be nil")
	}

	idStr, ok := id.(string)
	if !ok {
		return errors.New("id must be string")
	}

	doc, exists := m.documents[idStr]
	if !exists {
		return errors.New("no documents founds")
	}

	// Simple mock update logic
	if updateMap, ok := update.(map[string]any); ok {
		if name, exists := updateMap["name"]; exists {
			doc.Name = name.(string)
		}
		if email, exists := updateMap["email"]; exists {
			doc.Email = email.(string)
		}
		doc.Modified = time.Now()
	}

	return nil
}

func (m *MockMongoRepository) DeleteById(ctx context.Context, id any) error {
	if id == nil {
		return errors.New("id cannot be nil")
	}

	idStr, ok := id.(string)
	if !ok {
		return errors.New("id must be string")
	}

	_, exists := m.documents[idStr]
	if !exists {
		return errors.New("no documents founds")
	}

	delete(m.documents, idStr)
	return nil
}

func (m *MockMongoRepository) Count(ctx context.Context, filterBuilder *FilterBuilder) (int64, error) {
	return int64(len(m.documents)), nil
}

func (m *MockMongoRepository) Exists(ctx context.Context, id any) (bool, error) {
	if id == nil {
		return false, errors.New("id cannot be nil")
	}

	doc, err := m.FindById(ctx, id, nil)
	if err != nil {
		return false, err
	}
	return doc != nil, nil
}

// Test Suite
func TestMongoRepositoryFind(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *MockMongoRepository
		filter   *FilterBuilder
		expected int
		wantErr  bool
	}{
		{
			name: "find all documents",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["1"] = &TestModel{ID: "1", Name: "Test1", Email: "test1@example.com"}
				repo.documents["2"] = &TestModel{ID: "2", Name: "Test2", Email: "test2@example.com"}
				return repo
			},
			filter:   nil,
			expected: 2,
			wantErr:  false,
		},
		{
			name: "find with empty repository",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			filter:   nil,
			expected: 0,
			wantErr:  false,
		},
		{
			name: "find with limit",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["1"] = &TestModel{ID: "1", Name: "Test1"}
				repo.documents["2"] = &TestModel{ID: "2", Name: "Test2"}
				repo.documents["3"] = &TestModel{ID: "3", Name: "Test3"}
				return repo
			},
			filter:   NewFilter().Limit(2),
			expected: 2,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			results, err := repo.Find(ctx, tt.filter)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, results, tt.expected)
		})
	}
}

func TestMongoRepositoryFindOne(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *MockMongoRepository
		filter   *FilterBuilder
		expected *TestModel
		wantErr  bool
	}{
		{
			name: "find existing document",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["1"] = &TestModel{ID: "1", Name: "Test1", Email: "test1@example.com"}
				return repo
			},
			filter:   nil,
			expected: &TestModel{ID: "1", Name: "Test1", Email: "test1@example.com"},
			wantErr:  false,
		},
		{
			name: "find in empty repository",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			filter:   nil,
			expected: nil,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			result, err := repo.FindOne(ctx, tt.filter)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.Name, result.Name)
				assert.Equal(t, tt.expected.Email, result.Email)
			}
		})
	}
}

func TestMongoRepositoryFindById(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *MockMongoRepository
		id       any
		expected *TestModel
		wantErr  bool
	}{
		{
			name: "find existing document by id",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["test-id"] = &TestModel{ID: "test-id", Name: "Test", Email: "test@example.com"}
				return repo
			},
			id:       "test-id",
			expected: &TestModel{ID: "test-id", Name: "Test", Email: "test@example.com"},
			wantErr:  false,
		},
		{
			name: "find non-existing document",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:       "non-existing",
			expected: nil,
			wantErr:  false,
		},
		{
			name: "nil id",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:       nil,
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			result, err := repo.FindById(ctx, tt.id, nil)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.Name, result.Name)
			}
		})
	}
}

func TestMongoRepositoryInsert(t *testing.T) {
	tests := []struct {
		name    string
		doc     *TestModel
		wantErr bool
	}{
		{
			name: "successful insert",
			doc: &TestModel{
				Name:  "Test User",
				Email: "test@example.com",
				Age:   25,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := NewMockMongoRepository()
			ctx := context.Background()

			insertedID, err := repo.Insert(ctx, tt.doc)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, insertedID)
			assert.NotEmpty(t, tt.doc.ID)
			assert.False(t, tt.doc.Created.IsZero())
			assert.False(t, tt.doc.Modified.IsZero())
		})
	}
}

func TestMongoRepositoryCreate(t *testing.T) {
	repo := NewMockMongoRepository()
	ctx := context.Background()

	doc := &TestModel{
		Name:  "Test User",
		Email: "test@example.com",
		Age:   30,
	}

	result, err := repo.Create(ctx, doc)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "Test User", result.Name)
	assert.Equal(t, "test@example.com", result.Email)
	assert.Equal(t, 30, result.Age)
}

func TestMongoRepositoryUpdateById(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *MockMongoRepository
		id      any
		update  any
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful update",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["test-id"] = &TestModel{ID: "test-id", Name: "Original", Email: "original@example.com"}
				return repo
			},
			id:      "test-id",
			update:  map[string]any{"name": "Updated", "email": "updated@example.com"},
			wantErr: false,
		},
		{
			name: "nil id",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:      nil,
			update:  map[string]any{"name": "Updated"},
			wantErr: true,
			errMsg:  "id cannot be nil",
		},
		{
			name: "nil update",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:      "test-id",
			update:  nil,
			wantErr: true,
			errMsg:  "update cannot be nil",
		},
		{
			name: "document not found",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:      "non-existing",
			update:  map[string]any{"name": "Updated"},
			wantErr: true,
			errMsg:  "no documents founds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			err := repo.UpdateById(ctx, tt.id, tt.update)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)

			// Verify the update
			if tt.id != nil {
				doc, _ := repo.FindById(ctx, tt.id, nil)
				if updateMap, ok := tt.update.(map[string]any); ok {
					if name, exists := updateMap["name"]; exists {
						assert.Equal(t, name, doc.Name)
					}
				}
			}
		})
	}
}

func TestMongoRepositoryDeleteById(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *MockMongoRepository
		id      any
		wantErr bool
		errMsg  string
	}{
		{
			name: "successful delete",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["test-id"] = &TestModel{ID: "test-id", Name: "Test"}
				return repo
			},
			id:      "test-id",
			wantErr: false,
		},
		{
			name: "nil id",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:      nil,
			wantErr: true,
			errMsg:  "id cannot be nil",
		},
		{
			name: "document not found",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:      "non-existing",
			wantErr: true,
			errMsg:  "no documents founds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			err := repo.DeleteById(ctx, tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				return
			}

			require.NoError(t, err)

			// Verify deletion
			doc, _ := repo.FindById(ctx, tt.id, nil)
			assert.Nil(t, doc)
		})
	}
}

func TestMongoRepositoryCount(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *MockMongoRepository
		expected int64
	}{
		{
			name: "count with documents",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["1"] = &TestModel{ID: "1", Name: "Test1"}
				repo.documents["2"] = &TestModel{ID: "2", Name: "Test2"}
				return repo
			},
			expected: 2,
		},
		{
			name: "count empty repository",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			count, err := repo.Count(ctx, nil)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestMongoRepositoryExists(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() *MockMongoRepository
		id       any
		expected bool
		wantErr  bool
	}{
		{
			name: "document exists",
			setup: func() *MockMongoRepository {
				repo := NewMockMongoRepository()
				repo.documents["test-id"] = &TestModel{ID: "test-id", Name: "Test"}
				return repo
			},
			id:       "test-id",
			expected: true,
			wantErr:  false,
		},
		{
			name: "document does not exist",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:       "non-existing",
			expected: false,
			wantErr:  false,
		},
		{
			name: "nil id",
			setup: func() *MockMongoRepository {
				return NewMockMongoRepository()
			},
			id:       nil,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := tt.setup()
			ctx := context.Background()

			exists, err := repo.Exists(ctx, tt.id)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, exists)
		})
	}
}

// Benchmark tests
func BenchmarkMongoRepositoryFind(b *testing.B) {
	repo := NewMockMongoRepository()
	ctx := context.Background()

	// Setup test data
	for i := 0; i < 1000; i++ {
		id := fmt.Sprintf("id_%d", i)
		repo.documents[id] = &TestModel{
			ID:    id,
			Name:  fmt.Sprintf("Test%d", i),
			Email: fmt.Sprintf("test%d@example.com", i),
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := repo.Find(ctx, nil)
		if err != nil {
			b.Fatal(err)
		}
	}
}

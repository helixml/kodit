package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFilterOperator_String(t *testing.T) {
	tests := []struct {
		op   FilterOperator
		want string
	}{
		{OpEqual, "="},
		{OpNotEqual, "!="},
		{OpGreaterThan, ">"},
		{OpGreaterThanOrEqual, ">="},
		{OpLessThan, "<"},
		{OpLessThanOrEqual, "<="},
		{OpLike, "LIKE"},
		{OpILike, "ILIKE"},
		{OpIn, "IN"},
		{OpNotIn, "NOT IN"},
		{OpIsNull, "IS NULL"},
		{OpIsNotNull, "IS NOT NULL"},
		{OpBetween, "BETWEEN"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("FilterOperator.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortDirection_String(t *testing.T) {
	if SortAsc.String() != "ASC" {
		t.Errorf("SortAsc.String() = %v, want ASC", SortAsc.String())
	}
	if SortDesc.String() != "DESC" {
		t.Errorf("SortDesc.String() = %v, want DESC", SortDesc.String())
	}
}

func TestNewFilter(t *testing.T) {
	f := NewFilter("name", OpEqual, "test")

	if f.Field() != "name" {
		t.Errorf("Field() = %v, want name", f.Field())
	}
	if f.Operator() != OpEqual {
		t.Errorf("Operator() = %v, want OpEqual", f.Operator())
	}
	if f.Value() != "test" {
		t.Errorf("Value() = %v, want test", f.Value())
	}
}

func TestNewBetweenFilter(t *testing.T) {
	f := NewBetweenFilter("age", 18, 65)

	if f.Field() != "age" {
		t.Errorf("Field() = %v, want age", f.Field())
	}
	if f.Operator() != OpBetween {
		t.Errorf("Operator() = %v, want OpBetween", f.Operator())
	}
	if f.Value() != 18 {
		t.Errorf("Value() = %v, want 18", f.Value())
	}
}

func TestNewOrderBy(t *testing.T) {
	o := NewOrderBy("created_at", SortDesc)

	if o.Field() != "created_at" {
		t.Errorf("Field() = %v, want created_at", o.Field())
	}
	if o.Direction() != SortDesc {
		t.Errorf("Direction() = %v, want SortDesc", o.Direction())
	}
}

func TestQuery_Chaining(t *testing.T) {
	q := NewQuery().
		Equal("status", "active").
		GreaterThan("age", 18).
		In("role", []string{"admin", "user"}).
		OrderDesc("created_at").
		Limit(10).
		Offset(20)

	filters := q.Filters()
	if len(filters) != 3 {
		t.Errorf("expected 3 filters, got %d", len(filters))
	}

	orders := q.Orders()
	if len(orders) != 1 {
		t.Errorf("expected 1 order, got %d", len(orders))
	}

	if q.LimitValue() != 10 {
		t.Errorf("LimitValue() = %v, want 10", q.LimitValue())
	}

	if q.OffsetValue() != 20 {
		t.Errorf("OffsetValue() = %v, want 20", q.OffsetValue())
	}
}

func TestQuery_Paginate(t *testing.T) {
	tests := []struct {
		page     int
		pageSize int
		wantLim  int
		wantOff  int
	}{
		{1, 10, 10, 0},
		{2, 10, 10, 10},
		{3, 25, 25, 50},
		{0, 10, 10, 0},  // page < 1 defaults to 1
		{1, 0, 10, 0},   // pageSize < 1 defaults to 10
		{-1, -5, 10, 0}, // both invalid default
	}

	for _, tt := range tests {
		q := NewQuery().Paginate(tt.page, tt.pageSize)
		if q.LimitValue() != tt.wantLim {
			t.Errorf("Paginate(%d, %d) limit = %d, want %d", tt.page, tt.pageSize, q.LimitValue(), tt.wantLim)
		}
		if q.OffsetValue() != tt.wantOff {
			t.Errorf("Paginate(%d, %d) offset = %d, want %d", tt.page, tt.pageSize, q.OffsetValue(), tt.wantOff)
		}
	}
}

func TestQuery_AllFilterTypes(t *testing.T) {
	q := NewQuery().
		Equal("a", 1).
		NotEqual("b", 2).
		GreaterThan("c", 3).
		GreaterThanOrEqual("d", 4).
		LessThan("e", 5).
		LessThanOrEqual("f", 6).
		Like("g", "%test%").
		ILike("h", "%TEST%").
		In("i", []int{1, 2, 3}).
		NotIn("j", []int{4, 5, 6}).
		IsNull("k").
		IsNotNull("l").
		WhereBetween("m", 10, 20)

	filters := q.Filters()
	if len(filters) != 13 {
		t.Errorf("expected 13 filters, got %d", len(filters))
	}

	expectedOps := []FilterOperator{
		OpEqual, OpNotEqual, OpGreaterThan, OpGreaterThanOrEqual,
		OpLessThan, OpLessThanOrEqual, OpLike, OpILike,
		OpIn, OpNotIn, OpIsNull, OpIsNotNull, OpBetween,
	}

	for i, filter := range filters {
		if filter.Operator() != expectedOps[i] {
			t.Errorf("filter %d: Operator() = %v, want %v", i, filter.Operator(), expectedOps[i])
		}
	}
}

func TestQuery_OrderMethods(t *testing.T) {
	q := NewQuery().
		OrderAsc("name").
		OrderDesc("created_at").
		Order("updated_at", SortAsc)

	orders := q.Orders()
	if len(orders) != 3 {
		t.Fatalf("expected 3 orders, got %d", len(orders))
	}

	if orders[0].Field() != "name" || orders[0].Direction() != SortAsc {
		t.Errorf("order 0: got %s %v, want name ASC", orders[0].Field(), orders[0].Direction())
	}
	if orders[1].Field() != "created_at" || orders[1].Direction() != SortDesc {
		t.Errorf("order 1: got %s %v, want created_at DESC", orders[1].Field(), orders[1].Direction())
	}
	if orders[2].Field() != "updated_at" || orders[2].Direction() != SortAsc {
		t.Errorf("order 2: got %s %v, want updated_at ASC", orders[2].Field(), orders[2].Direction())
	}
}

func TestQuery_Apply(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	err = db.Session(ctx).Exec(`
		CREATE TABLE test_users (
			id INTEGER PRIMARY KEY,
			name TEXT,
			age INTEGER,
			status TEXT
		)
	`).Error
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert test data
	err = db.Session(ctx).Exec(`
		INSERT INTO test_users (name, age, status) VALUES
		('Alice', 30, 'active'),
		('Bob', 25, 'inactive'),
		('Charlie', 35, 'active')
	`).Error
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	// Test Query.Apply
	q := NewQuery().
		Equal("status", "active").
		GreaterThan("age", 28).
		OrderDesc("age").
		Limit(10)

	type User struct {
		ID     int64
		Name   string
		Age    int
		Status string
	}

	var users []User
	result := q.Apply(db.Session(ctx).Table("test_users")).Find(&users)
	if result.Error != nil {
		t.Fatalf("query: %v", result.Error)
	}

	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}

	// Should be ordered by age DESC
	if len(users) >= 2 {
		if users[0].Name != "Charlie" {
			t.Errorf("expected first user to be Charlie, got %s", users[0].Name)
		}
		if users[1].Name != "Alice" {
			t.Errorf("expected second user to be Alice, got %s", users[1].Name)
		}
	}
}

func TestQuery_ApplyWithBetween(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	// Create a test table
	err = db.Session(ctx).Exec(`
		CREATE TABLE test_products (
			id INTEGER PRIMARY KEY,
			name TEXT,
			price INTEGER
		)
	`).Error
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	// Insert test data
	err = db.Session(ctx).Exec(`
		INSERT INTO test_products (name, price) VALUES
		('Widget', 50),
		('Gadget', 100),
		('Gizmo', 150)
	`).Error
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	q := NewQuery().WhereBetween("price", 50, 100)

	type Product struct {
		ID    int64
		Name  string
		Price int
	}

	var products []Product
	result := q.Apply(db.Session(ctx).Table("test_products")).Find(&products)
	if result.Error != nil {
		t.Fatalf("query: %v", result.Error)
	}

	if len(products) != 2 {
		t.Errorf("expected 2 products, got %d", len(products))
	}
}

func TestQuery_ApplyWithIn(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	url := "sqlite:///" + dbPath

	db, err := NewDatabase(ctx, url)
	if err != nil {
		t.Fatalf("NewDatabase: %v", err)
	}
	defer func() { _ = db.Close() }()

	err = db.Session(ctx).Exec(`CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)`).Error
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	err = db.Session(ctx).Exec(`INSERT INTO test_items (name) VALUES ('a'), ('b'), ('c'), ('d')`).Error
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	q := NewQuery().In("name", []string{"a", "c"})

	type Item struct {
		ID   int64
		Name string
	}

	var items []Item
	result := q.Apply(db.Session(ctx).Table("test_items")).Find(&items)
	if result.Error != nil {
		t.Fatalf("query: %v", result.Error)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

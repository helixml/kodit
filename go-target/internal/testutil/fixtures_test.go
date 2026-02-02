package testutil

import (
	"context"
	"errors"
	"testing"
)

func TestTestDatabase(t *testing.T) {
	db := TestDatabase(t)

	ctx := context.Background()
	var result int
	err := db.Session(ctx).Raw("SELECT 1").Scan(&result).Error
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected result 1, got %d", result)
	}
}

func TestTestConfig(t *testing.T) {
	cfg := TestConfig(t)

	if cfg.DataDir() == "" {
		t.Error("expected non-empty DataDir")
	}
	if cfg.DBURL() == "" {
		t.Error("expected non-empty DBURL")
	}
	if cfg.LogLevel() != "DEBUG" {
		t.Errorf("LogLevel() = %v, want DEBUG", cfg.LogLevel())
	}
}

func TestTestDatabaseWithSchema(t *testing.T) {
	ctx := context.Background()
	schema := `CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)`
	db := TestDatabaseWithSchema(t, schema)

	// Verify table exists by inserting data
	err := db.Session(ctx).Exec("INSERT INTO test_items (name) VALUES (?)", "test").Error
	if err != nil {
		t.Fatalf("insert failed: %v", err)
	}
}

func TestPtr(t *testing.T) {
	intPtr := Ptr(42)
	if *intPtr != 42 {
		t.Errorf("Ptr(42) = %d, want 42", *intPtr)
	}

	strPtr := Ptr("hello")
	if *strPtr != "hello" {
		t.Errorf("Ptr(\"hello\") = %s, want hello", *strPtr)
	}
}

func TestMust(t *testing.T) {
	result := Must(42, nil)
	if result != 42 {
		t.Errorf("Must(42, nil) = %d, want 42", result)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Must should panic on error")
		}
	}()
	Must(0, errors.New("test error"))
}

func TestAssertEqual(t *testing.T) {
	// Create a mock testing.T to capture failures
	mockT := &testing.T{}

	AssertEqual(mockT, 1, 1, "should be equal")
	if mockT.Failed() {
		t.Error("AssertEqual should not fail for equal values")
	}
}

func TestAssertNoError(t *testing.T) {
	mockT := &testing.T{}

	AssertNoError(mockT, nil, "should not error")
	if mockT.Failed() {
		t.Error("AssertNoError should not fail for nil error")
	}
}

func TestAssertError(t *testing.T) {
	mockT := &testing.T{}

	AssertError(mockT, errors.New("test"), "should error")
	if mockT.Failed() {
		t.Error("AssertError should not fail for non-nil error")
	}
}

func TestAssertLen(t *testing.T) {
	mockT := &testing.T{}

	slice := []int{1, 2, 3}
	AssertLen(mockT, slice, 3, "should have length 3")
	if mockT.Failed() {
		t.Error("AssertLen should not fail for correct length")
	}
}

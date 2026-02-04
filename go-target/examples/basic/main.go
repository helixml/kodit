// Package main demonstrates basic usage of the kodit library.
//
// This example shows how to:
// - Create a kodit client with SQLite storage
// - Clone and index a repository
// - Perform a hybrid search
// - Query enrichments
//
// To run this example:
//
//	export OPENAI_API_KEY=your_api_key
//	go run main.go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/helixml/kodit"
)

func main() {
	ctx := context.Background()

	// Create a temporary directory for the example
	tmpDir, err := os.MkdirTemp("", "kodit-example-*")
	if err != nil {
		log.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	dbPath := filepath.Join(tmpDir, "kodit.db")

	// Create a new kodit client with SQLite storage
	// The background worker starts automatically
	client, err := kodit.New(
		kodit.WithSQLite(dbPath),
		kodit.WithDataDir(tmpDir),
		// Uncomment to enable OpenAI for enrichments and vector search:
		// kodit.WithOpenAI(os.Getenv("OPENAI_API_KEY")),
	)
	if err != nil {
		log.Fatalf("failed to create client: %v", err)
	}
	defer func() { _ = client.Close() }()

	fmt.Println("Kodit client created successfully")

	// List repositories (should be empty initially)
	repos, err := client.Repositories().List(ctx)
	if err != nil {
		log.Fatalf("failed to list repositories: %v", err)
	}
	fmt.Printf("Initial repositories: %d\n", len(repos))

	// List pending tasks (should be empty initially)
	tasks, err := client.Tasks().List(ctx)
	if err != nil {
		log.Fatalf("failed to list tasks: %v", err)
	}
	fmt.Printf("Pending tasks: %d\n", len(tasks))

	// Perform a search (returns empty results without indexed content)
	results, err := client.Search(ctx, "create deployment",
		kodit.WithSemanticWeight(0.7),
		kodit.WithLimit(10),
	)
	if err != nil {
		log.Fatalf("failed to search: %v", err)
	}
	fmt.Printf("Search results: %d snippets\n", results.Count())

	// Example: Clone a repository (uncomment to actually clone)
	// This will queue the repository for indexing by the background worker
	//
	// repo, err := client.Repositories().Clone(ctx, "https://github.com/gorilla/mux")
	// if err != nil {
	//     log.Fatalf("failed to clone repository: %v", err)
	// }
	// fmt.Printf("Cloned repository: %d - %s\n", repo.ID(), repo.RemoteURL())
	//
	// // Wait for indexing to complete
	// time.Sleep(30 * time.Second)
	//
	// // Search the indexed repository
	// results, err := client.Search(ctx, "route handler",
	//     kodit.WithRepositories(repo.ID()),
	//     kodit.WithLimit(5),
	// )
	// if err != nil {
	//     log.Fatalf("failed to search: %v", err)
	// }
	// for _, snippet := range results.Snippets() {
	//     fmt.Printf("Found: %s in %s\n", snippet.Name(), snippet.Path())
	// }

	fmt.Println("Example completed successfully")
	_ = time.Second // suppress unused import warning when example code is commented
}

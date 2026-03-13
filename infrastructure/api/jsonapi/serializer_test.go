package jsonapi

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/helixml/kodit/domain/repository"
)

func TestRepositoryResource_SanitizesCredentials(t *testing.T) {
	repo := repository.ReconstructRepository(
		1,
		"http://user:secret-token@api:8080/git/my-repo",
		"http://api:8080/git/my-repo",
		"",
		repository.WorkingCopy{},
		repository.NewTrackingConfigForBranch("main"),
		time.Now(), time.Now(), time.Time{},
	)
	source := repository.NewSource(repo)

	serializer := NewSerializer()
	resource := serializer.RepositoryResource(source)

	data, err := json.Marshal(resource)
	if err != nil {
		t.Fatalf("marshal resource: %v", err)
	}

	body := string(data)
	if strings.Contains(body, "secret-token") {
		t.Errorf("RepositoryResource leaks credentials: %s", body)
	}
	if !strings.Contains(body, "http://api:8080/git/my-repo") {
		t.Errorf("expected sanitized URL in output, got: %s", body)
	}
}

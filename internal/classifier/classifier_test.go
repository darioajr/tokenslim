package classifier

import "testing"

func TestDetect(t *testing.T) {
	for cmd, want := range map[string]string{"./mvnw test": "maven", "mvn test": "maven", "./gradlew build": "gradle", "npm run test": "node-test", "npx jest": "node-test", "python -m pytest": "pytest", "go test ./...": "go-test", "cargo test": "rust-test", "kubectl logs pod": "kubernetes-log", "docker compose logs": "docker-log", "terraform plan": "terraform", "ansible-playbook play.yml": "ansible", "cat README.md": "generic", "notmvn test": "generic"} {
		if got := Detect(cmd, ""); got != want {
			t.Errorf("%s: %s != %s", cmd, got, want)
		}
	}
}

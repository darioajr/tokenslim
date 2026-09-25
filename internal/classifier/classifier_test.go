package classifier

import "testing"

func TestDetect(t *testing.T) {
	for cmd, want := range map[string]string{"./mvnw test": "maven", "mvn test": "maven", "./gradlew build": "gradle", "npm run test": "node-test", "npx jest": "node-test", "python -m pytest": "pytest", "go test ./...": "go-test", "cargo test": "rust-test", "kubectl logs pod": "kubernetes-log", "docker compose logs": "docker-log", "terraform plan": "terraform", "ansible-playbook play.yml": "ansible", "cat README.md": "generic", "notmvn test": "generic"} {
		if got := Detect(cmd, ""); got != want {
			t.Errorf("%s: %s != %s", cmd, got, want)
		}
	}
}

func TestPHPCommands(t *testing.T) {
	for cmd, want := range map[string]string{
		"php -l app/Models/User.php":                     "php",
		"/usr/bin/php8.4 script.php":                     "php",
		"php -d memory_limit=-1 artisan test --parallel": "laravel",
		"./artisan migrate --force":                      "laravel",
		"./vendor/bin/sail test":                         "laravel",
		"sail artisan queue:work":                        "laravel",
		"vendor/bin/phpunit --testdox":                   "php-test",
		"php phpunit.phar":                               "php-test",
		"./vendor/bin/pest --parallel":                   "php-test",
		"composer exec pest":                             "php-test",
		"composer install --no-interaction":              "composer",
		"php composer.phar update":                       "composer",
		"composer test":                                  "composer",
		"notphp script.php":                              "generic",
		"phpunit-wrapper":                                "generic",
		"artisan-notes":                                  "generic",
		"sailboat":                                       "generic",
	} {
		if got := Detect(cmd, ""); got != want {
			t.Errorf("%q: got %s, want %s", cmd, got, want)
		}
	}
}

func TestPHPReduction(t *testing.T) {
	for _, command := range []string{"php artisan test", "php -d memory_limit=-1 artisan test --parallel", "php8.4 -n -c php.ini artisan --env=testing test", "./vendor/bin/sail test --parallel", "sail artisan test", "sail pest", "sail phpunit", "vendor/bin/phpunit --testdox", "php phpunit.phar", "vendor/bin/pest --parallel"} {
		if got := PHPReduction(command); got != "php-test" {
			t.Errorf("%s: %q", command, got)
		}
	}
	for _, command := range []string{"composer install", "php composer.phar update", "composer --no-interaction update"} {
		if got := PHPReduction(command); got != "composer" {
			t.Errorf("%s: %q", command, got)
		}
	}
	for _, command := range []string{"", "php", "php -d", "php -r code", "sail", "php artisan --env testing migrate", "php artisan custom:test", "composer run-script update", "cat vendor/bin/phpunit", "echo php artisan test", "php artisan test | tee tests.log", "composer install && composer audit"} {
		if got := PHPReduction(command); got != "" {
			t.Errorf("%s: %q", command, got)
		}
	}
}
